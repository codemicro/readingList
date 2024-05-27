#!/usr/bin/env python3
import uuid
import sqlite3
import csv
import sys

CSVFILE = sys.argv[1]
SQLITEFILE = sys.argv[2]

with open(CSVFILE) as csvf:
    csv_reader = csv.reader(csvf)

    x = list(map(lambda x: (str(uuid.uuid4()), *map(lambda y: (y if y != "" else ""), x)), list(csv_reader)[1:]))
    db = sqlite3.connect(SQLITEFILE)
    db.executemany(
        """INSERT INTO articles("id", "url", "title", "description", "image_url", "date", "hacker_news_url") VALUES (?, ?, ?, ?, ?, ?, ?)""",
        x,
    )
    db.commit()