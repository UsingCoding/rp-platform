# Build locally first: mise run //modules/payment:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/payment /payment
EXPOSE 8081 8082
ENTRYPOINT ["/payment"]
CMD ["service"]
