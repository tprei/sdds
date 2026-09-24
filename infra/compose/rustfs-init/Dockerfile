FROM golang:1.24.7-alpine3.22@sha256:fc2cff6625f3c1c92e6c85938ac5bd09034ad0d4bc2dfb08278020b68540dbb5 AS mc-build
RUN apk add --no-cache git && git clone --depth 1 --branch RELEASE.2025-08-13T08-35-41Z https://github.com/minio/mc /src && cd /src && go build -trimpath -o /usr/bin/mc .
FROM amazon/aws-cli:2.27.47@sha256:9c09286acba40b27577e630dc6aff4cc44b401eaa81ebda69d3c761bb566def2
COPY --from=mc-build /usr/bin/mc /usr/local/bin/mc
COPY init.sh /usr/local/bin/rustfs-init
COPY secret-file.sh /usr/local/lib/rustfs-init/secret-file.sh
COPY validate-secrets.sh /usr/local/bin/validate-compose-secrets
COPY api-policy.json /etc/rustfs-init/api-policy.json
RUN chmod 0555 /usr/local/bin/rustfs-init /usr/local/bin/validate-compose-secrets /usr/local/bin/mc && chmod 0444 /usr/local/lib/rustfs-init/secret-file.sh /etc/rustfs-init/api-policy.json
ENTRYPOINT ["/usr/local/bin/rustfs-init"]
