# NSQLite Contributing Guidelines

## Thank You

Thank you for contributing to the NSQLite project! We appreciate all contributions, whether they involve writing code, writing tests, improving documentation, or providing feedback. Your efforts help make this project better for everyone. Together, we can create a high-quality and reliable tool.

Any contribution is welcomed and valued, no matter how small.

---

## How to Contribute

1. **Branching and Pull Requests:**
   - The development branch is `develop`, so base your work on this branch since the `main` branch is reserved for releases.
   - Create a new branch for each feature or fix.
   - Name branches descriptively, e.g., `feature/add-user-auth` or `fix/resolve-db-connection`.
   - Submit pull requests (PRs) to the `develop` branch, not the `main` branch.

2. **General Guidelines:**
   - Ensure your contributions align with the project's goals and existing conventions.
   - Be respectful and collaborative in discussions and code reviews.

---

## Project Rules

To maintain consistency and readability across the project, follow these rules:

### 1. Code Style
   - Write idiomatic Go code by following the [Effective Go guidelines](https://golang.org/doc/effective_go.html).
   - Use tools like `gofmt` to ensure your code meets Go standards.

### 2. Naming Conventions
   - Use `camelCase` for all identifiers, including:
     - JSON keys
     - Query parameters
     - Form values
     - Variable and function names
   - Avoid meaningless variable names like `a`, `b`, `c`, `x`, `y`, `z`.
   - Use descriptive and meaningful names to make the code self-explanatory.

### 3. Code Formatting
   - Run `gofmt` before committing your code to ensure consistent formatting.
   - Keep functions and files small and focused on a single responsibility.
   - Write clean, self-explanatory code to minimize the need for comments.

### 4. Comments
   - Write useful comments that add value and context.
   - Avoid redundant comments that merely restate the code.
   - Document all exported functions, methods, and packages using Go's standard comment conventions.

### 5. Git Commit Messages
   - Use present-tense imperative verbs (e.g., "Add feature," not "Added feature").
   - Keep messages concise and informative.
   - Include issue or ticket references when applicable.

### 6. Testing
   - Write unit tests for all new features and bug fixes.
   - Use Go's built-in testing framework (`testing` package).
   - Ensure all tests pass locally before submitting a pull request.

### 7. Dependencies
   - Minimize the addition of new dependencies.
   - Clearly document and justify any new dependencies added.

### 8. Best Practices
   - Refactor code to improve readability and maintainability when necessary.
   - Avoid premature optimization; prioritize clarity and correctness first.

---

By following these guidelines, we ensure that NSQLite remains clean, maintainable, and accessible to all contributors. Thank you for your dedication!
