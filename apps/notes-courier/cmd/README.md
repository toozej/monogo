# `/cmd`

This directory contains the main applications for this project.

Prefer a directory name that matches the executable name. For example, use `/cmd/notes-courier`.

Keep the application directory small. Put code that other projects can import and reuse in `/pkg`.
Put private code or code that you do not want others to reuse in `/internal`.
Other developers can find unexpected uses for code, so state your intended use explicitly.

A common pattern uses a small `main` function. This function only imports and invokes code from `/internal` and `/pkg`.
