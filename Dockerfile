FROM amd64/debian:12-slim

COPY ./dist/CalculatorAPI-linux-amd64 /app/CalculatorAPI

RUN apt-get update && apt-get upgrade -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    chmod +x /app/CalculatorAPI

EXPOSE 12345

CMD ["/app/CalculatorAPI"]