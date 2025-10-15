# Accessing Git Submodules from the GitHub Web UI

When a Git repository includes a submodule, GitHub's web interface treats the submodule as a special entry:

1. The main repository shows the submodule as a directory with a curved arrow icon.
2. Clicking the submodule entry takes you to the external repository (or a specific commit snapshot) referenced by the submodule pointer.
3. You cannot browse the submodule's contents inline within the parent repository on GitHub; instead, you view the linked repository directly.
4. If the referenced commit lives in a private repository you cannot access, GitHub displays a 404 or an access denied message.
5. To inspect files inline, you must clone the repository locally with `--recurse-submodules` or initialize the submodule after cloning.

> **TL;DR**: The GitHub frontend lets you follow a link to the submodule repository/commit, but it does not render the submodule's files inside the parent repository's file tree.
