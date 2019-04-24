FROM BASEIMAGE
RUN apk --no-cache add ca-certificates bash

ARG ARCH
ARG TINI_VERSION

ADD gitlab-controller /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["gitlab-controller"]
