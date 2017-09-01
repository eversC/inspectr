FROM ubuntu:16.10
RUN apt-get install ca-certificates
ADD main /main
ENTRYPOINT ["/main"]