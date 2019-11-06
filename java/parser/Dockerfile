# Note: I used the official maven Docker image to build this package.
# However, Maven complains it cannot find tools.jar. I verified that
# I'd have to yum install java-8-openjdk in that Docker image to make
# Maven works.  However, I'd follow this alternative to create a
# Docker image from Ubuntu instead of Oracle Linux.
FROM ubuntu:18.04

RUN apt-get update && apt-get install -y openjdk-8-jdk maven openjfx
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

