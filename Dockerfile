FROM gcr.io/distroless/static-debian12

COPY sshark-api /usr/bin/sshark-api
COPY db/migrations /db/migrations

ENTRYPOINT [ "/usr/bin/sshark-api" ]

CMD ["serve"]
