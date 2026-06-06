FROM python:3.11-alpine

RUN pip install requests httpx

WORKDIR /app

COPY client.py /app/

CMD ["python", "client.py"]