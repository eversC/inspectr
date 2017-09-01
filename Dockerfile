FROM ubuntu:16.10
ADD main /main
ENTRYPOINT ["/main"]