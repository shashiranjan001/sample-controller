FROM ubuntu
WORKDIR /
COPY sample-controller .

EXPOSE 8000
ENTRYPOINT [ "/sample-controller" ]
