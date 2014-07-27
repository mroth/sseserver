go.rice is used to bundle the view templates into the package.  After making any
changes to a HTML template, do the following to build:

    go get github.com/GeertJohan/go.rice/rice
    $GOPATH/bin/rice embed-go
    go build

Be sure to check the corresponding changes in with `views.rice-box.go` then to
match the template changes. This is annoying but I haven't found a better
flexible way to do this in Go yet.
