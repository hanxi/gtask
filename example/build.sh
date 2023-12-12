go build -buildmode=plugin -o myplugin.so plugin-services/myplugin/service_main.go
go build -buildmode=plugin -o myplugin2.so plugin-services/myplugin2/service_main.go
go build -buildmode=plugin -o bootstrap.so plugin-services/bootstrap/service_main.go
go build
