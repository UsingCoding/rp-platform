# Build locally first: mise run //modules/order:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/order /order
EXPOSE 8081 8082
ENTRYPOINT ["/order"]
CMD ["service"]
