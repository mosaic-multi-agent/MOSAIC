"""Reads the complete lines of the OTel file from an offset, bounded by a time budget.

This module mirrors Tools/Common/otelingest/lines.go.
Standard library only; deployed flat next to the adapter modules.
"""

from mosaic_logger_otel_parse import USAGE_MARKER, parse_line


class Scanned:
    """What one read of the OTel file produced."""

    def __init__(self, start):
        self.lines = []
        self.read = 0
        self.malformed = 0
        self.end = start  # offset after the last complete line consumed
        self.cut = False  # reading stopped at a line boundary because the budget ran out


def _follows_newline(handle, offset):
    """Whether the byte before offset is a newline, i.e. offset is at the start of a line."""
    handle.seek(offset - 1)
    byte = handle.read(1)
    handle.seek(offset)
    return byte == b"\n"


def scan_file(path, start, out_of_time):
    """Read the complete lines of the file from start. A trailing partial line is not read and not
    consumed. Lines without the usage marker are skipped without JSON parsing. out_of_time is asked
    before each complete line is consumed; when it answers True the scan stops before that line."""
    out = Scanned(start)
    with open(path, "rb") as handle:
        handle.seek(start)
        if start > 0 and not _follows_newline(handle, start):
            skipped = handle.readline()
            if not skipped.endswith(b"\n"):
                return out
            out.end += len(skipped)
        while True:
            line = handle.readline()
            if not line.endswith(b"\n"):
                return out
            if out_of_time():
                out.cut = True
                return out
            offset = out.end
            out.end += len(line)
            out.read += 1
            if USAGE_MARKER not in line:
                continue
            parsed = parse_line(line, offset)
            if parsed.malformed:
                out.malformed += 1
            elif parsed.link is not None or parsed.call is not None or parsed.tool is not None:
                out.lines.append(parsed)
