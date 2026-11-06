# Build locally first: mise run //modules/delivery:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/delivery /delivery
EXPOSE 8081 8082
ENTRYPOINT ["/delivery"]
CMD ["service"]
