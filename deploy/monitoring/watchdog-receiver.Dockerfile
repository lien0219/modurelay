FROM python:3.13.8-alpine3.22
RUN addgroup -g 65532 receiver && adduser -D -H -u 65532 -G receiver receiver \
    && mkdir /state && chown 65532:65532 /state
COPY watchdog_receiver.py /app/watchdog_receiver.py
USER 65532:65532
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
EXPOSE 8080
CMD ["python", "/app/watchdog_receiver.py"]
