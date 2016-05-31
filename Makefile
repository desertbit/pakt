# Helper Makefile

all:
	go build

test:
	# Run the test twice. One normal test routine and one with the race detection enabled.
	go test
	go test -race
