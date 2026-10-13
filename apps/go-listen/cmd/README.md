# `/cmd`

Main applications for this project.

The directory name for each application should match the name of the executable you want to have (e.g., `/cmd/go-listen`).

Do not put a lot of code in the application directory. If you think the code can be imported and used in other projects, then it should live in the `/pkg` directory. If the code is not reusable or if you do not want others to reuse it, put that code in the `/internal` directory. You will be surprised what others will do, so be explicit about your intentions!

It is common to have a small `main` function that imports and invokes the code from the `/internal` and `/pkg` directories and nothing else.
