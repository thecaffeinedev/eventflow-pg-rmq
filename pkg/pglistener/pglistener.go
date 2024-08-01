package pglistener

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
)

type PgListener struct {
	conn *pgconn.PgConn
}

func New(connString string) (*PgListener, error) {
	conn, err := pgconn.Connect(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return &PgListener{conn: conn}, nil
}

func (pl *PgListener) Listen(ctx context.Context) (<-chan models.Event, error) {
	err := createPublication(ctx, pl.conn)
	if err != nil {
		return nil, fmt.Errorf("error creating publication: %v", err)
	}

	slotName := "cdc_slot"
	err = createReplicationSlot(ctx, pl.conn, slotName)
	if err != nil {
		return nil, fmt.Errorf("error creating replication slot: %v", err)
	}

	eventChan := make(chan models.Event)

	go func() {
		defer close(eventChan)

		err := startReplication(ctx, pl.conn, slotName, eventChan)
		if err != nil {
			log.Printf("Error in replication: %v", err)
		}
	}()

	return eventChan, nil
}

func createPublication(ctx context.Context, conn *pgconn.PgConn) error {
	result := conn.Exec(ctx, "CREATE PUBLICATION all_tables FOR ALL TABLES")
	err := result.Close()
	if err != nil {
		// Check if the error is because the publication already exists
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42710" {
			// Publication already exists, log and continue
			log.Printf("Publication 'all_tables' already exists")
			return nil
		}
		return fmt.Errorf("error creating publication: %v", err)
	}
	return nil
}

func createReplicationSlot(ctx context.Context, conn *pgconn.PgConn, slotName string) error {
	_, err := pglogrepl.CreateReplicationSlot(ctx, conn, slotName, "pgoutput", pglogrepl.CreateReplicationSlotOptions{Temporary: false})
	if err != nil {
		// Check if the error is because the slot already exists
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42710" {
			log.Printf("Replication slot '%s' already exists", slotName)
			return nil
		}
		return fmt.Errorf("error creating replication slot: %v", err)
	}
	return nil
}

func startReplication(ctx context.Context, conn *pgconn.PgConn, slotName string, eventChan chan<- models.Event) error {
	err := pglogrepl.StartReplication(ctx, conn, slotName, 0, pglogrepl.StartReplicationOptions{
		PluginArgs: []string{
			"proto_version '1'",
			"publication_names 'all_tables'",
		},
	})
	if err != nil {
		return fmt.Errorf("error starting replication: %v", err)
	}

	clientXLogPos := pglogrepl.LSN(0)
	standbyMessageTimeout := time.Second * 10

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ctx, cancel := context.WithTimeout(ctx, standbyMessageTimeout)
		rawMsg, err := conn.ReceiveMessage(ctx)
		cancel()
		if err != nil {
			if pgconn.Timeout(err) {
				err = sendStandbyStatusUpdate(ctx, conn, clientXLogPos)
				if err != nil {
					return fmt.Errorf("error sending standby status update: %v", err)
				}
				continue
			}
			return fmt.Errorf("error receiving message: %v", err)
		}

		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			return fmt.Errorf("error from Postgres: %s", errMsg.Message)
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		switch msg.Data[0] {
		case pglogrepl.XLogDataByteID:
			xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
			if err != nil {
				return fmt.Errorf("error parsing XLogData: %v", err)
			}
			clientXLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))

			event, err := parseWALData(xld.WALData)
			if err != nil {
				log.Printf("Error parsing WAL data: %v", err)
				continue
			}
			eventChan <- event

		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if err != nil {
				return fmt.Errorf("error parsing PrimaryKeepaliveMessage: %v", err)
			}
			if pkm.ReplyRequested {
				err = sendStandbyStatusUpdate(ctx, conn, clientXLogPos)
				if err != nil {
					return fmt.Errorf("error sending standby status update: %v", err)
				}
			}
		}
	}
}

func sendStandbyStatusUpdate(ctx context.Context, conn *pgconn.PgConn, clientXLogPos pglogrepl.LSN) error {
	return pglogrepl.SendStandbyStatusUpdate(ctx, conn, pglogrepl.StandbyStatusUpdate{
		WALWritePosition: clientXLogPos,
	})
}

func parseWALData(data []byte) (models.Event, error) {
	var event models.Event
	err := json.Unmarshal(data, &event)
	if err != nil {
		return models.Event{}, fmt.Errorf("error unmarshaling event: %v", err)
	}
	return event, nil
}

func (pl *PgListener) Close(ctx context.Context) error {
	return pl.conn.Close(ctx)
}
