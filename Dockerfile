FROM golang:1.4

ENV API_KEY=212af9729e854cb3b4d2715978527575
ENV PORT=3210
ENV TLS_CERT=tls-cert.pem
ENV TLS_KEY=tls-key.pem

RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN GOPATH=/app/Godeps/_workspace go build -o htflood .

CMD ["sh", "-c", "/app/htflood bot -api-key ${API_KEY} -port ${PORT} -tls-key ${TLS_KEY} -tls-cert ${TLS_CERT}" ]

EXPOSE ${PORT}
