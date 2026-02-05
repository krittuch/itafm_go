CREATE TABLE IF NOT EXISTS flight_flight (
    id SERIAL PRIMARY KEY,
    aircraft TEXT,
    type TEXT,
    schedule_flight_time TIMESTAMPTZ,
    flight_number TEXT,
    next_station TEXT,
    prev_station TEXT,
    ac_register TEXT,
    bay TEXT,
    gate TEXT,
    tobt TEXT,
    actual_flight_time TIMESTAMPTZ,
    estimate_flight_time TIMESTAMPTZ,
    canceled BOOLEAN DEFAULT FALSE,
    working BOOLEAN DEFAULT FALSE,
    finished BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

TRUNCATE TABLE flight_flight;

INSERT INTO flight_flight (
    aircraft,
    type,
    schedule_flight_time,
    flight_number,
    next_station,
    prev_station,
    ac_register,
    bay,
    tobt,
    created_at,
    updated_at
) VALUES (
    'A320',
    'DEP',
    NOW(),
    'TG 123',
    'VTSP',
    'VTBS',
    'HS-ABC',
    '',
    '',
    NOW(),
    NOW()
);
