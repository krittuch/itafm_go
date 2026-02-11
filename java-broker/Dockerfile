FROM maven:3.9.8-eclipse-temurin-17 AS build

WORKDIR /build
COPY java-broker/pom.xml ./pom.xml
COPY java-broker/src ./src
RUN mvn -q -DskipTests package

FROM eclipse-temurin:17-jre

WORKDIR /app
COPY --from=build /build/target/activemq-kafka-broker.jar /app/activemq-kafka-broker.jar

ENTRYPOINT ["java", "--add-modules", "jdk.httpserver", "-jar", "/app/activemq-kafka-broker.jar"]
