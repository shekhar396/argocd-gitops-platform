# Kubernetes database migrations

`migrations/001_init.sql` is the schema source of truth. Build
`Dockerfile.migrations` from the repository root: it copies `migrations/` into
`/migrations/` in a PostgreSQL 17 image, which supplies `psql`. The Job runs the
image's default command against the `postgres` Service in `gitops-demo`.
Credentials come from the existing `postgres-secret` keys `POSTGRES_DB`,
`POSTGRES_USER`, and `POSTGRES_PASSWORD`, mapped to psql's environment variables.
No SQL or credentials are embedded in the Job manifest.

The command runs `001_init.sql` with `ON_ERROR_STOP=1` and
`--single-transaction`: an SQL error fails the Job and rolls back the migration.
The current file creates only missing tables/indexes, so rerunning it preserves
existing rows. This does not reconcile an existing table with a different schema.
The Job retries failed Pods up to three times and has a five-minute deadline.

The PostgreSQL StatefulSet now mounts only the existing PVC. Applying its change
restarts the PostgreSQL Pod, causing a brief database interruption, but retains
the PVC and database. The old init ConfigMap can remain unused in the cluster;
its manifest has been removed from the repository.

## Manual test on kind (PowerShell)

Run from the repository root with Docker and kind running. These commands use
the explicit `kind-cka-lab` context and assume the namespace, Secrets, Service,
StatefulSet, and PVC already exist. Stop if any command fails.

```powershell
docker build -f Dockerfile.migrations -t argocd-gitops-migrations:v1 .
kind load docker-image argocd-gitops-migrations:v1 --name cka-lab

kubectl --context kind-cka-lab -n gitops-demo apply -f k8s/postgres-statefulset.yaml
kubectl --context kind-cka-lab -n gitops-demo rollout status statefulset/postgres --timeout=180s

# Capture existing row counts before the migration (no credentials printed).
'SELECT count(*) AS users_count FROM users; SELECT count(*) AS tasks_count FROM tasks;' | kubectl --context kind-cka-lab -n gitops-demo exec -i postgres-0 -- sh -c 'psql -X -w -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

kubectl --context kind-cka-lab -n gitops-demo apply -f k8s/postgres-migration-job.yaml
kubectl --context kind-cka-lab -n gitops-demo wait --for=condition=complete job/postgres-migration --timeout=330s
kubectl --context kind-cka-lab -n gitops-demo logs job/postgres-migration

# Inspect schema and compare row counts with the earlier output.
@'
\d users
\d tasks
SELECT count(*) AS users_count FROM users;
SELECT count(*) AS tasks_count FROM tasks;
'@ | kubectl --context kind-cka-lab -n gitops-demo exec -i postgres-0 -- sh -c 'psql -X -w -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

Expect notices that the existing tables/index already exist, followed by a
Completed Job and unchanged row counts if the API has received no concurrent
writes. For a failure, inspect:

```powershell
kubectl --context kind-cka-lab -n gitops-demo describe job postgres-migration
kubectl --context kind-cka-lab -n gitops-demo get pods -l app=postgres-migration
kubectl --context kind-cka-lab -n gitops-demo logs job/postgres-migration
```

To test idempotency, delete **only the migration Job** and run it again. Applying
a completed Job does not rerun it; Job Pod templates are immutable.

```powershell
kubectl --context kind-cka-lab -n gitops-demo delete job postgres-migration --ignore-not-found=true --wait=true
kubectl --context kind-cka-lab -n gitops-demo apply -f k8s/postgres-migration-job.yaml
kubectl --context kind-cka-lab -n gitops-demo wait --for=condition=complete job/postgres-migration --timeout=330s
kubectl --context kind-cka-lab -n gitops-demo logs job/postgres-migration
```

For a fresh deployment, create PostgreSQL and its dependencies, wait for it,
run the migration Job to completion, then deploy the API. Kubernetes does not
automatically gate the API on this Job; apply files in that order rather than
applying the entire directory together. Compose continues using the same source
SQL through its existing bind mount for first-time database initialization.

This is an explicit runner for `001_init.sql`, without migration history or
automatic discovery. Future files are included in the image but are not executed
automatically. Before adding non-idempotent migrations, introduce version tracking
and ordered execution. Rebuild with a new image tag, load it into kind, update
the Job image, and recreate only the Job for each image change. Avoid concurrent
migration Jobs. Argo CD integration is intentionally deferred.
