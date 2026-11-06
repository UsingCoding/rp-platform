# Build locally first: mise run //modules/ordercreator:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/ordercreator /ordercreator
EXPOSE 8081 8082
ENTRYPOINT ["/ordercreator"]
CMD ["service"]
