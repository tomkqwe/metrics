CREATE TABLE gauges (
    id text PRIMARY KEY,
    value double precision NOT NULL
);

CREATE TABLE counters (
    id text PRIMARY KEY,
    value bigint NOT NULL
);
