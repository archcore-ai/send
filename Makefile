# Archcore Send — skill (client) build/test entrypoints.
# The skill ships no binaries; these targets only lint and test shell scripts.

SKILL_DIR  := skill/send
SEND_SH    := $(SKILL_DIR)/scripts/send.sh
BATS       := bats
SERVER_DIR := server
SENDD_BIN  := tests/bin/sendd

.PHONY: help lint test test-unit test-e2e verify-no-binaries doctor \
        server-build server-vet server-test sendd-e2e docker-build test-all

help:
	@printf '%s\n' \
	  'Archcore Send — skill targets:' \
	  '  make lint                 shellcheck send.sh' \
	  '  make test-unit            bats unit tests (stubbed age/curl)' \
	  '  make test-e2e             bats e2e round-trip vs python mock server' \
	  '  make test                 unit + e2e' \
	  '  make verify-no-binaries   assert the skill tree contains no binaries' \
	  '  make doctor               run send.sh doctor' \
	  '' \
	  'Server (sendd) targets:' \
	  '  make server-build         build the static sendd binary into tests/bin/' \
	  '  make server-vet           go vet ./...' \
	  '  make server-test          go test ./... -race' \
	  '  make sendd-e2e            bats e2e against the real Go server' \
	  '  make docker-build         build the sendd container image' \
	  '  make test-all             skill tests + server tests + sendd-e2e'

lint:
	shellcheck $(SEND_SH)

test-unit:
	$(BATS) tests/unit.bats

test-e2e:
	$(BATS) tests/e2e.bats

test: test-unit test-e2e

# INV-4: the installed skill directory must contain no binaries / compiled artifacts.
# Text scripts report as "... executable" via file(1) but are fine; only true
# compiled objects (Mach-O / ELF / PE / static archive) count as binaries.
verify-no-binaries:
	@found=$$(find $(SKILL_DIR) -type f -exec sh -c \
	  'case "$$(file -b "$$1")" in *Mach-O*|*ELF*|*PE32*|*"current ar archive"*|*"compiled"*) echo "$$1";; esac' _ {} \;); \
	if [ -n "$$found" ]; then \
	  printf 'BINARY ARTIFACTS FOUND in %s:\n%s\n' "$(SKILL_DIR)" "$$found" >&2; exit 1; \
	else \
	  printf 'OK: no binaries under %s\n' "$(SKILL_DIR)"; \
	fi

doctor:
	bash $(SEND_SH) doctor

# --- Server (sendd) ---

server-build:
	cd $(SERVER_DIR) && CGO_ENABLED=0 go build -o ../$(SENDD_BIN) .

server-vet:
	cd $(SERVER_DIR) && go vet ./...

server-test:
	cd $(SERVER_DIR) && go test ./... -race -count=1

sendd-e2e: server-build
	SEND_BACKEND=sendd $(BATS) tests/e2e.bats

docker-build:
	docker build -t sendd:local $(SERVER_DIR)

test-all: test server-test sendd-e2e
