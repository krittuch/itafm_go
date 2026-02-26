CREATE TABLE IF NOT EXISTS aods_broker_messages (
    id BIGSERIAL PRIMARY KEY,
    stream TEXT NOT NULL CHECK (stream IN ('FLMO', 'IDEP')),
    kafka_topic TEXT NOT NULL,
    command TEXT,
    payload TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_aods_broker_messages_stream_received_at
    ON aods_broker_messages (stream, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_aods_broker_messages_command_received_at
    ON aods_broker_messages (command, received_at DESC);
