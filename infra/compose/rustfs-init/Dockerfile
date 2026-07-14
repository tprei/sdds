FROM amazon/aws-cli:2.27.47@sha256:9c09286acba40b27577e630dc6aff4cc44b401eaa81ebda69d3c761bb566def2
COPY --from=minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727 /usr/bin/mc /usr/local/bin/mc
COPY init.sh /usr/local/bin/rustfs-init
COPY api-policy.json /etc/rustfs-init/api-policy.json
RUN chmod 0555 /usr/local/bin/rustfs-init /usr/local/bin/mc && chmod 0444 /etc/rustfs-init/api-policy.json
ENTRYPOINT ["/usr/local/bin/rustfs-init"]
