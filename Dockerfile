FROM gcr.io/distroless/static-debian12

COPY sshark-api /usr/bin/sshark-api

ENTRYPOINT [ "/usr/bin/sshark-api" ]

CMD ["serve"]

