package pglistener

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
)

type PgListener struct {
	dsn    string
	replcn *pgconn.PgConn
	cn     *pgxpool.Pool
	slot   string
	events []string
}

func New(dsn string) (*PgListener, error) {
	return &PgListener{
		dsn:    dsn,
		events: []string{"insert", "update", "delete"},
	}, nil
}

func (pl *PgListener) connect(ctx context.Context) error {
	config, err := pgxpool.ParseConfig(pl.dsn)
	if err != nil {
		return err
	}
	config.MaxConns = 5
	config.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	pl.cn = pool

	// Modify the DSN for replication connection
	replDSN := pl.dsn
	if strings.Contains(replDSN, "?") {
		replDSN += "&replication=database"
	} else {
		replDSN += "?replication=database"
	}

	replcn, err := pgconn.Connect(ctx, replDSN)
	if err != nil {
		return err
	}
	pl.replcn = replcn
	return nil
}

func (pl *PgListener) createPublication(ctx context.Context) error {
	_, err := pl.cn.Exec(ctx, "CREATE PUBLICATION all_tables FOR ALL TABLES")
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Println("Publication 'all_tables' already exists")
			return nil
		}
		return err
	}
	log.Println("Publication 'all_tables' created successfully")
	return nil
}

func (pl *PgListener) createReplicationSlot(ctx context.Context) error {
	_, err := pglogrepl.CreateReplicationSlot(ctx, pl.replcn, pl.slot, "pgoutput", pglogrepl.CreateReplicationSlotOptions{Temporary: false})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("Replication slot '%s' already exists", pl.slot)
			return nil
		}
		return err
	}
	log.Printf("Replication slot '%s' created successfully", pl.slot)
	return nil
}

func (pl *PgListener) Listen(ctx context.Context) (<-chan models.Event, error) {
	err := pl.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	err = pl.createPublication(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create publication: %v", err)
	}

	pl.slot = "cdc_slot"
	err = pl.createReplicationSlot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create replication slot: %v", err)
	}

	eventChan := make(chan models.Event)

	go func() {
		defer close(eventChan)
		err := pl.startReplication(ctx, eventChan)
		if err != nil {
			log.Printf("Error in replication: %v", err)
		}
	}()

	return eventChan, nil
}

func (pl *PgListener) startReplication(ctx context.Context, eventChan chan<- models.Event) error {
	err := pglogrepl.StartReplication(ctx, pl.replcn, pl.slot, 0, pglogrepl.StartReplicationOptions{
		PluginArgs: []string{
			"proto_version '1'",
			"publication_names 'all_tables'",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to start replication: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pl.handleReplication(ctx, eventChan)
	}()

	wg.Wait()
	return nil
}

func (pl *PgListener) handleReplication(ctx context.Context, eventChan chan<- models.Event) {
	tables := make(map[uint32]string)
	columns := make(map[uint32][]string)

	for ctx.Err() == nil {
		msg, err := pl.replcn.ReceiveMessage(ctx)
		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			log.Printf("Failed to receive message: %v", err)
			break
		}

		switch msg := msg.(type) {
		case *pgproto3.CopyData:
			switch msg.Data[0] {
			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					log.Printf("Failed to parse XLogData: %v", err)
					continue
				}

				logicalMsg, err := pglogrepl.Parse(xld.WALData)
				if err != nil {
					log.Printf("Failed to parse logical replication message: %v", err)
					continue
				}

				switch logicalMsg := logicalMsg.(type) {
				case *pglogrepl.RelationMessage:
					tables[logicalMsg.RelationID] = fmt.Sprintf("public.%s", logicalMsg.RelationName)
					columns[logicalMsg.RelationID] = make([]string, len(logicalMsg.Columns))
					for i, col := range logicalMsg.Columns {
						columns[logicalMsg.RelationID][i] = col.Name
					}

				case *pglogrepl.InsertMessage:
					tableName := tables[logicalMsg.RelationID]
					newValues := parseValues(columns[logicalMsg.RelationID], logicalMsg.Tuple)
					event := models.Event{
						Action:    "I",
						TableName: tableName,
						Old:       map[string]interface{}{},
						New:       newValues,
						Diff:      newValues,
					}
					if id, ok := newValues["id"]; ok {
						if idInt, ok := id.(float64); ok {
							event.ID = int(idInt)
						}
					}
					eventChan <- event

				case *pglogrepl.UpdateMessage:
					tableName := tables[logicalMsg.RelationID]
					oldValues := parseValues(columns[logicalMsg.RelationID], logicalMsg.OldTuple)
					newValues := parseValues(columns[logicalMsg.RelationID], logicalMsg.NewTuple)
					diffValues := make(map[string]interface{})
					for k, v := range newValues {
						if oldValues[k] != v {
							diffValues[k] = v
						}
					}
					event := models.Event{
						Action:    "U",
						TableName: tableName,
						Old:       oldValues,
						New:       newValues,
						Diff:      diffValues,
					}
					if id, ok := newValues["id"]; ok {
						if idInt, ok := id.(float64); ok {
							event.ID = int(idInt)
						}
					}
					eventChan <- event

				case *pglogrepl.DeleteMessage:
					tableName := tables[logicalMsg.RelationID]
					oldValues := parseValues(columns[logicalMsg.RelationID], logicalMsg.OldTuple)
					event := models.Event{
						Action:    "D",
						TableName: tableName,
						Old:       oldValues,
						New:       map[string]interface{}{},
						Diff:      map[string]interface{}{},
					}
					if id, ok := oldValues["id"]; ok {
						if idInt, ok := id.(float64); ok {
							event.ID = int(idInt)
						}
					}
					eventChan <- event
				}

			case pglogrepl.PrimaryKeepaliveMessageByteID:
				_, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
				if err != nil {
					log.Printf("Failed to parse primary keepalive message: %v", err)
					continue
				}
				err = pglogrepl.SendStandbyStatusUpdate(ctx, pl.replcn, pglogrepl.StandbyStatusUpdate{WALWritePosition: pglogrepl.LSN(0)})
				if err != nil {
					log.Printf("Failed to send standby status update: %v", err)
				}
			}
		}
	}
}
func parseValues(columns []string, tuple *pglogrepl.TupleData) map[string]interface{} {
	values := make(map[string]interface{})
	for i, col := range columns {
		if i < len(tuple.Columns) {
			values[col] = string(tuple.Columns[i].Data)
		}
	}
	return values
}

func (pl *PgListener) Close() error {
	if pl.cn != nil {
		pl.cn.Close()
	}
	if pl.replcn != nil {
		return pl.replcn.Close(context.Background())
	}
	return nil
}
