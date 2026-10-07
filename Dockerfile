# Build the manager and backup-worker binaries
FROM golang:1.27.1 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/manager/main.go
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o backup-worker cmd/backup-worker/main.go

# backup-worker shells out to the real pg_dump binary, which distroless
# cannot provide (no shell, no package manager) - so, unlike the
# manager-only image this replaces, the final stage needs a real Linux
# userland. debian-slim + the upstream PostgreSQL APT repo (not Debian's
# own default postgresql-client, which tracks an older major version) is
# the standard way to get a current pg_dump client. This is a real,
# deliberate reduction in hardening versus distroless, signed off on for
# OEBC-2249.
FROM debian:bookworm-slim

# pg_dump's client major version must be >= the highest Postgres server
# major version this operator backs up (pg_dump requires client >= server,
# never the reverse) - 17 is current stable and gives headroom above
# today's targets. Bump this if a newer server major version is ever
# introduced.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates gnupg curl \
    && install -d /usr/share/postgresql-common/pgdg \
    && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
    && apt-get update && apt-get install -y --no-install-recommends postgresql-client-17 \
    && apt-get purge -y --auto-remove gnupg curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 1000 --no-create-home --shell /usr/sbin/nologin nonroot

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/backup-worker .

# UID 1000, not distroless's 65532: the dump/cleanup CronJobs' pod security
# context already hardcodes runAsUser: 1000 (internal/controller/pgbackuppolicy_controller.go),
# so the image needs that exact UID to exist for backup-worker's own
# in-container identity to resolve cleanly; the manager Deployment's
# securityContext only requires *some* non-root UID, so 1000 works there too.
USER 1000:1000

ENTRYPOINT ["/manager"]
