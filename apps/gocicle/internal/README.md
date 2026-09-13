# Internal packages

The job schema and browser UI do not import storage or credential packages.
The service package applies authorization and database transactions.
The runner package handles host workspaces and reports ordered events.
The runtime package provides Docker and Podman adapters.
The providers package separates login and repository metadata from Git transport.
The translate package parses workflows without executing them.
