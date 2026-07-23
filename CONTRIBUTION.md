# Contributing to Molx

Thanks for your interest in making Molx better. This project is a small, focused HTTP service, so we keep contributions practical and to the point.

---

## Getting started

You need Go 1.23 or later and the Obscura binaries for your platform.

```bash
# Download Obscura binaries for your OS/arch
make setup

# Run the server locally
API_KEY=your-secret-key go run .
```

Open `http://localhost:8080/reference` to explore the interactive API docs.

---

## What to contribute

- Bug fixes for existing scrapers, search engines, or handlers
- Improvements to content extraction and cleanup
- Better test coverage
- Documentation fixes
- Small, focused features that fit the project's scope

Before working on a large change, open an issue first so we can discuss the direction.

---

## How to contribute

1. Fork the repository and create a branch for your change.
2. Make your edits. Keep the code simple and consistent with the existing style.
3. Add or update tests when it makes sense.
4. Run the test suite and make sure everything passes:

   ```bash
   make test
   ```

5. Run the linter:

   ```bash
   make lint
   ```

6. Open a pull request with a clear description of what changed and why.

---

## Code style

- Go code follows standard `gofmt` formatting.
- Keep functions small and packages focused.
- Prefer clarity over cleverness.
- Return explicit errors instead of swallowing them.
- Add tests for new behavior, especially around scraping and search logic.

---

## Commit messages

Write short, descriptive commit messages. A good format:

```
fix(handler): return proper error when URL is missing
```

Use the imperative mood (`fix`, `add`, `update`) and keep the summary under 72 characters.

---

## Reporting issues

When reporting a bug, include:

- What you were trying to do
- The URL or input that caused the issue
- The expected result and the actual result
- Steps to reproduce, if possible

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
