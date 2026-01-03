---
name: blogwatcher-cli
description: Use when managing or interacting with favorite blogs via the BlogWatcher CLI—adding/removing blogs, scanning for new posts, listing articles, marking read/unread, or modifying related CLI behavior, scanning, storage, and tests.
---

# BlogWatcher CLI

## Quick Orientation
- Use the Click entry point in `blogwatcher/cli.py`.
- Route business logic through `blogwatcher/controllers.py` and persistence through `blogwatcher/db.py`.
- Use scanning pipeline modules in `blogwatcher/scanner.py`, `blogwatcher/rss.py`, and `blogwatcher/scraper.py`.
- Remember the default SQLite path is `~/.blogwatcher/blogwatcher.db` and is created on demand.

## Run Commands
- Prefer `uv run blogwatcher ...` when working locally.
- Alternatively run `uv run python -m blogwatcher.cli ...`.
- Use the `blogwatcher` script when installed via the project scripts entry point.

## Change Workflow
1. Add or adjust CLI commands in `blogwatcher/cli.py` (Click options, arguments, and output formatting).
2. Put non-trivial logic in `blogwatcher/controllers.py` so CLI stays thin and testable.
3. Update storage or schema in `blogwatcher/db.py` and adjust model conversion in `blogwatcher/models.py` if needed.
4. Modify scanning behavior in `blogwatcher/scanner.py` and its helpers (`blogwatcher/rss.py`, `blogwatcher/scraper.py`).
5. Update or add tests under `tests/` for every feature change or addition.

## Test Guidance
- Run tests with `uv run pytest`.
- If you add a feature, add tests and any necessary dummy data.
- Keep tests focused on CLI behavior (click invocations), controller logic, and scraper/RSS parsing outcomes.

## Output Conventions
- Preserve user-friendly CLI output with colors and clear errors.
- When listing posts available for reading, always include the link to each post in the output.
- Keep error handling consistent with existing exceptions (`BlogNotFoundError`, `BlogAlreadyExistsError`, `ArticleNotFoundError`).

### Example (posts available for reading)
```text
Unread articles (2):

  [12] [new] Understanding Click Contexts
       Blog: Real Python
       URL: https://realpython.com/click-context/
       Published: 2025-11-02

  [13] [new] Async IO in Practice
       Blog: Test & Code
       URL: https://testandcode.com/async-io-in-practice/
```
