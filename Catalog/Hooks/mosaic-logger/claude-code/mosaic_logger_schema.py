"""mosaic_logger_schema.py - Per-event schema version stamping.

Most events carry the base schema version; events whose shape was extended in
a later log format release carry that release's version instead.
"""

BASE_SCHEMA_VERSION = "1.3.0"

EVENT_SCHEMA_VERSIONS = {
    "compaction": "1.5.0",
}


def schema_version_for(event: str) -> str:
    """Return the schema version an event of this type is stamped with."""
    return EVENT_SCHEMA_VERSIONS.get(event, BASE_SCHEMA_VERSION)
