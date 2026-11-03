CREATE TABLE IF NOT EXISTS docs (
    collection TEXT    NOT NULL,
    id         INTEGER NOT NULL,
    body       TEXT    NOT NULL,
    PRIMARY KEY (collection, id)
);

CREATE TABLE IF NOT EXISTS kv (
    key  TEXT PRIMARY KEY,
    body TEXT NOT NULL
);

DROP TABLE IF EXISTS torrents;
