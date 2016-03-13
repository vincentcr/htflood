FROM golang:1.6
RUN go get github.com/tools/godep

ENV API_KEY=212af9729e854cb3b4d2715978527575
ENV PORT=3210
ENV TLS_CERT=tls-cert.pem
ENV TLS_KEY=tls-key.pem

ENV APP_DIR=$GOPATH/src/github.com/vincentcr/htflood

RUN mkdir -p $APP_DIR
WORKDIR $APP_DIR
COPY . $APP_DIR
RUN godep go build -o htflood .

CMD ["sh", "-c", "$APP_DIR/htflood bot --port=${PORT} --tls-key=${TLS_KEY} --tls-cert=${TLS_CERT} ${API_KEY}" ]

EXPOSE ${PORT}
