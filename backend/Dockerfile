FROM --platform=linux/amd64 heroiclabs/nakama-pluginbuilder:3.22.0 AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOTOOLCHAIN=local

WORKDIR /build

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN cd cmd/plugin && go build --trimpath --buildmode=plugin -o /out/backend.so .

FROM --platform=linux/amd64 heroiclabs/nakama:3.22.0

COPY --from=builder /out/backend.so /nakama/data/modules/backend.so
