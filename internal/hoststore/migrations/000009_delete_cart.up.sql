CREATE TABLE delete_cart (
    object_hash TEXT PRIMARY KEY,
    sender TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    mailed_at TEXT NOT NULL DEFAULT '',
    tldr TEXT NOT NULL DEFAULT '',
    added_at TEXT NOT NULL
);
