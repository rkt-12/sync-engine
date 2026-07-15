Commands - 
1. ```gofmt -l .``` -> to checks the formatting issues of the whole directory without modifying any files. If it prints nothing, it means that formatting is correct
2. ```gofmt -w .``` -> to automatically fix all the formatting issues
3. ```go vet ./...``` -> this is a static analysis tool that looks for likely bugs and suspicious code in Go program. If it prints nothing, means no bugs.
4. ```go test -v ./internal/crdt/...``` -> builds and runs all the tests in the internal/crdt package and any of its subpackages , -v for verbose.
5. ```go test -race ./internal/crdt/...``` -> in this -race checks if there are no unsynchronized concurrent reads or writes to shared memory. this is specifically for clock (PS - this didnt run due to system issues)

Notes- 
1. Any function whose name starts with Test is automatically a test function.