# story: e01s02
# Build a static binary (no cgo) and ship it on distroless as non-root.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Generated templ code is committed, so no generate step here.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/upareal ./cmd/upareal \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/upareal /upareal
# A new named volume mounted at /data inherits this owner, so uid 65532 can write.
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/upareal"]
