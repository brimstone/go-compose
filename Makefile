compose: main.go
	clear
	CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags '-s' -o compose
	ls -lh compose

test: compose
	./compose < test.yaml
