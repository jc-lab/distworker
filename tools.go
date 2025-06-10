//go:build tools
// +build tools

package distworker

//go:generate swag i -d ./go/pkg/controller/ -g server.go -o ./go/cmd/controller/docs/ --parseDependency --parseInternal
