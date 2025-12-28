FROM scratch

COPY sshark-api /usr/bin/sshark-api

ENTRYPOINT [ "/usr/bin/sshark-api" ]

CMD ["serve"]

