package app

import (
	"database/sql"
	"log"
)

type aodsArchiveStore struct {
	db *sql.DB
}

func newAODSArchiveStore(db *sql.DB) *aodsArchiveStore {
	return &aodsArchiveStore{db: db}
}

func (s *aodsArchiveStore) enabled() bool {
	return s != nil && s.db != nil
}

func (s *aodsArchiveStore) SaveFLMO(topic string, payload []byte, command string) {
	s.save("FLMO", topic, payload, command)
}

func (s *aodsArchiveStore) SaveIDEP(topic string, payload []byte) {
	s.save("IDEP", topic, payload, "")
}

func (s *aodsArchiveStore) save(stream string, topic string, payload []byte, command string) {
	if !s.enabled() || len(payload) == 0 {
		return
	}

	_, err := s.db.Exec(
		`INSERT INTO aods_broker_messages (stream, kafka_topic, command, payload)
		 VALUES ($1, $2, NULLIF($3, ''), $4)`,
		stream,
		topic,
		command,
		string(payload),
	)
	if err != nil {
		log.Printf("AODS archive save failed stream=%s topic=%s: %v", stream, topic, err)
	}
}
