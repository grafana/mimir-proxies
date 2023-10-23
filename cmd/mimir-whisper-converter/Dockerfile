FROM python:3.10-bullseye AS build

RUN apt update && apt -y install git \
  && git clone https://github.com/graphite-project/whisper.git /whisper

WORKDIR /whisper

RUN python ./setup.py build

FROM python:3.10-bullseye

RUN addgroup --gid 1000 app \
  && adduser --uid 1000 --home /app --gid 1000 --system app \
  && apt update && apt -y --no-install-recommends install prometheus vim groff

RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
  && unzip awscliv2.zip \
  && ./aws/install

RUN wget https://s3.amazonaws.com/mountpoint-s3-release/latest/x86_64/mount-s3.deb \
  && apt-get install -y ./mount-s3.deb

WORKDIR /app
USER app

COPY --from=build /whisper/build /app/opt/whisper
ENV PYTHONPATH=/app/opt/whisper/lib
ENV PATH=$PATH:/app/opt/whisper/scripts-3.10

COPY dist/mimir-whisper-converter /app/mimir-whisper-converter
COPY scripts/whisper /app/bin

CMD ["tail", "-f", "/dev/null"]
