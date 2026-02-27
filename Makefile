BIN_DIR=bin
BIN=$(BIN_DIR)/nusantarad

.PHONY: build test run fmt package

build:
	go build -o $(BIN) ./cmd/nusantarad

test:
	go test ./...

run:
	NUSANTARA_ALLOW_NON_UBUNTU=true NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=DevStrongPass123 go run ./cmd/nusantarad

fmt:
	go fmt ./...

package:
	bash ./scripts/package_release.sh
