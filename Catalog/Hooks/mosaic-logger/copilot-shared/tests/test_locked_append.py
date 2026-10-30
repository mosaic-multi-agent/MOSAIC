"""The locked append: one whole line is appended while an exclusive lock on the file is held.

A concurrent writer waits for the lock for a bounded time; a writer that cannot get it in
that time drops the line and reports it, without raising into the harness.
"""

import importlib
import pathlib
import time

import pytest

from locked_append_support import hold_lock, line_problems, lock_file_of, release_after, run_writers

HOOK_DIR = pathlib.Path(__file__).resolve().parents[2]
SHARED_DIR = pathlib.Path(__file__).resolve().parents[1]
MODULE = "mosaic_logger_locked_append"
LINE = b'{"event":"probe"}\n'
BOUND_SLACK = 6.0  # the wait limit is 2 s; generous margin for a loaded machine
RELEASE_AFTER = 0.5  # a lock released this long into the wait
WAITED_AT_LEAST = 0.4


@pytest.fixture
def la():
    return importlib.import_module(MODULE)


def test_parallel_writers_of_long_lines_lose_tear_or_blank_no_line_and_a_lock_free_reader_is_not_broken(tmp_path):
    target = tmp_path / "run" / "00_orchestrator_events.jsonl"

    # run_writers also reads the file without a lock all through the writes and reports
    # any read failure or torn line it sees
    problems = run_writers("locked", [SHARED_DIR], target)

    assert problems == []
    assert line_problems(target) == []


def test_the_wait_limit_and_stale_age_are_the_values_of_the_log_format(la):
    assert la.WAIT_LIMIT_SECONDS == 2.0
    assert la.STALE_AFTER_SECONDS == 10.0


def test_lock_path_is_the_dot_prefixed_sidecar_in_the_same_directory(la, tmp_path):
    target = tmp_path / "run" / "00_orchestrator_events.jsonl"

    assert pathlib.Path(la.lock_path(target)) == tmp_path / "run" / ".00_orchestrator_events.jsonl.lock"


def test_appends_the_line_creates_the_directory_and_leaves_no_lock_file(la, tmp_path):
    target = tmp_path / "new" / "dir" / "events.jsonl"

    written = la.locked_append(target, LINE)
    la.locked_append(target, LINE)

    assert written is True
    assert target.read_bytes() == LINE + LINE
    assert not lock_file_of(target).exists()


def test_a_held_lock_drops_the_line_within_the_bound_and_reports_once_without_raising(la, tmp_path):
    target = tmp_path / "events.jsonl"
    lock = hold_lock(target)
    reports = []

    started = time.monotonic()
    written = la.locked_append(target, LINE, lambda message, exc: reports.append((message, exc)))
    elapsed = time.monotonic() - started

    assert written is False
    assert not target.exists()
    assert lock.exists()
    assert elapsed < BOUND_SLACK
    assert len(reports) == 1
    assert reports[0][1] is None
    assert target.name in reports[0][0]


def test_a_held_lock_without_a_debug_function_still_returns_false(la, tmp_path):
    target = tmp_path / "events.jsonl"
    hold_lock(target)

    assert la.locked_append(target, LINE) is False


def test_a_writer_waits_for_a_lock_released_mid_wait_and_then_writes(la, tmp_path):
    target = tmp_path / "events.jsonl"
    hold_lock(target)
    reports = []
    release_after(lock_file_of(target), RELEASE_AFTER)

    started = time.monotonic()
    written = la.locked_append(target, LINE, lambda message, exc: reports.append(message))
    elapsed = time.monotonic() - started

    assert written is True
    assert target.read_bytes() == LINE
    assert elapsed >= WAITED_AT_LEAST
    assert reports == []
    assert not lock_file_of(target).exists()


def test_a_lock_a_few_seconds_old_is_not_stale_and_the_line_is_dropped(la, tmp_path):
    target = tmp_path / "events.jsonl"
    lock = hold_lock(target, age_seconds=5)

    written = la.locked_append(target, LINE)

    assert written is False
    assert not target.exists()
    assert lock.exists()


def test_a_lock_older_than_the_stale_age_is_broken_and_the_line_written(la, tmp_path):
    target = tmp_path / "events.jsonl"
    hold_lock(target, age_seconds=30)

    written = la.locked_append(target, LINE)

    assert written is True
    assert target.read_bytes() == LINE
    assert not lock_file_of(target).exists()


def test_an_io_error_returns_false_and_reports_the_exception_without_raising(la, tmp_path):
    blocker = tmp_path / "not_a_dir"
    blocker.write_bytes(b"")
    reports = []

    written = la.locked_append(blocker / "events.jsonl", LINE,
                               lambda message, exc: reports.append((message, exc)))

    assert written is False
    assert len(reports) == 1
    assert isinstance(reports[0][1], OSError)


def test_with_raise_os_errors_an_io_error_raises_after_releasing_the_lock(la, tmp_path):
    target = tmp_path / "is_a_directory"
    target.mkdir()
    reports = []

    with pytest.raises(OSError):
        la.locked_append(target, LINE, lambda message, exc: reports.append(message),
                         raise_os_errors=True)

    assert not lock_file_of(target).exists()
    assert reports == []


def test_with_raise_os_errors_a_timeout_still_returns_false(la, tmp_path):
    target = tmp_path / "events.jsonl"
    hold_lock(target)

    assert la.locked_append(target, LINE, raise_os_errors=True) is False


def test_holding_lock_yields_true_while_the_lock_file_exists_and_releases_on_exit(la, tmp_path):
    target = tmp_path / "events.jsonl"

    with la.holding_lock(target) as held:
        inside = lock_file_of(target).exists()

    assert held is True
    assert inside is True
    assert not lock_file_of(target).exists()


def test_holding_lock_yields_false_on_timeout_and_leaves_the_other_holders_lock(la, tmp_path):
    target = tmp_path / "events.jsonl"
    lock = hold_lock(target)
    reports = []

    started = time.monotonic()
    with la.holding_lock(target, lambda message, exc: reports.append(message)) as held:
        pass
    elapsed = time.monotonic() - started

    assert held is False
    assert lock.exists()
    assert elapsed < BOUND_SLACK
    assert len(reports) == 1


def test_holding_lock_waits_for_a_lock_released_mid_wait_and_then_yields_true(la, tmp_path):
    target = tmp_path / "events.jsonl"
    hold_lock(target)
    release_after(lock_file_of(target), RELEASE_AFTER)

    started = time.monotonic()
    with la.holding_lock(target) as held:
        inside = lock_file_of(target).exists()
    elapsed = time.monotonic() - started

    assert held is True
    assert inside is True
    assert elapsed >= WAITED_AT_LEAST
    assert not lock_file_of(target).exists()


def test_an_exception_from_the_block_propagates_after_the_lock_is_released(la, tmp_path):
    target = tmp_path / "events.jsonl"

    with pytest.raises(ValueError):
        with la.holding_lock(target):
            raise ValueError("block failed")

    assert not lock_file_of(target).exists()


def test_a_plain_append_inside_the_held_lock_block_is_not_blocked_by_that_lock(la, tmp_path):
    target = tmp_path / "events.jsonl"

    with la.holding_lock(target) as held:
        assert held is True
        with open(target, "ab") as handle:
            handle.write(LINE)

    assert target.read_bytes() == LINE


def test_the_shared_copy_is_identical_to_the_claude_code_copy():
    shared = (SHARED_DIR / (MODULE + ".py")).read_bytes()
    claude = (HOOK_DIR / "claude-code" / (MODULE + ".py")).read_bytes()

    assert shared == claude


def test_the_header_names_the_copy_it_must_stay_identical_to():
    head = (SHARED_DIR / (MODULE + ".py")).read_text(encoding="utf-8").splitlines()[:25]

    text = "\n".join(head)
    assert "claude-code" in text
    assert "copilot-shared" in text


def test_the_test_support_copies_are_identical_in_every_tests_folder():
    name = "locked_append_support.py"
    copies = [(HOOK_DIR / folder / "tests" / name).read_bytes()
              for folder in ("copilot-shared", "claude-code", "ghcp-cli", "vscode-ghcp")]

    assert len(set(copies)) == 1
