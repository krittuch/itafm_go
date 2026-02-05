package com.aerothai.itafm.broker;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.Properties;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import javax.jms.BytesMessage;
import javax.jms.Connection;
import javax.jms.Destination;
import javax.jms.JMSException;
import javax.jms.Message;
import javax.jms.MessageConsumer;
import javax.jms.Session;
import javax.jms.TextMessage;

import org.apache.activemq.ActiveMQConnectionFactory;
import org.apache.kafka.clients.producer.KafkaProducer;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.apache.kafka.clients.producer.RecordMetadata;
import org.apache.kafka.common.serialization.ByteArraySerializer;
import org.apache.kafka.common.serialization.StringSerializer;

public final class ActiveMqKafkaBridgeApp {
    private static final Duration RETRY_DELAY = Duration.ofSeconds(3);
    private static volatile boolean running = true;

    private ActiveMqKafkaBridgeApp() {
    }

    public static void main(String[] args) {
        BridgeConfig config = BridgeConfig.fromEnv();
        List<RouteConfig> routes = List.of(
            RouteConfig.fromRawDestination(
                "FLIGHT_MOVEMENT",
                readEnv("MQTT_FLIGHT_MOVEMENT_QUEUE", "/queue/FLMO_ITFM_Queue"),
                DestinationKind.QUEUE,
                readEnv("KAFKA_FLIGHT_TOPIC", "itafm.flight_movement")
            ),
            RouteConfig.fromRawDestination(
                "IDEP",
                readEnv("MQTT_IDEP_QUEUE", "/queue/IDEP_ITFM_Queue"),
                DestinationKind.QUEUE,
                readEnv("KAFKA_IDEP_TOPIC", "itafm.idep")
            ),
            RouteConfig.fromRawDestination(
                "SURVEILLANCE",
                readEnv("MQTT_SURV_TOPIC", "/topic/AS62_VTBB_ITFM2_Topic"),
                DestinationKind.TOPIC,
                readEnv("KAFKA_SURV_TOPIC", "itafm.surveillance")
            )
        );

        KafkaProducer<String, byte[]> producer = new KafkaProducer<>(config.kafkaProperties());
        ExecutorService executor = Executors.newFixedThreadPool(routes.size());

        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            running = false;
            System.out.println("Shutting down ActiveMQ -> Kafka broker...");
            executor.shutdownNow();
            producer.flush();
            producer.close(Duration.ofSeconds(10));
        }));

        for (RouteConfig route : routes) {
            executor.submit(() -> consumeForever(config, route, producer));
        }

        try {
            executor.awaitTermination(Long.MAX_VALUE, TimeUnit.DAYS);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
    }

    private static void consumeForever(BridgeConfig config, RouteConfig route, KafkaProducer<String, byte[]> producer) {
        while (running && !Thread.currentThread().isInterrupted()) {
            Connection connection = null;
            Session session = null;
            MessageConsumer consumer = null;

            try {
                ActiveMQConnectionFactory factory = new ActiveMQConnectionFactory(
                    config.activeMqUser,
                    config.activeMqPassword,
                    config.activeMqBrokerUrl
                );

                connection = factory.createConnection();
                connection.start();

                session = connection.createSession(false, Session.CLIENT_ACKNOWLEDGE);
                Destination destination = route.kind == DestinationKind.TOPIC
                    ? session.createTopic(route.destinationName)
                    : session.createQueue(route.destinationName);
                consumer = session.createConsumer(destination);

                System.out.printf(
                    "Route %s online (ActiveMQ %s:%s -> Kafka topic %s)%n",
                    route.label,
                    route.kind.name().toLowerCase(),
                    route.destinationName,
                    route.kafkaTopic
                );

                while (running && !Thread.currentThread().isInterrupted()) {
                    Message message = consumer.receive(1000L);
                    if (message == null) {
                        continue;
                    }

                    byte[] payload = extractPayload(message);
                    if (payload.length == 0) {
                        message.acknowledge();
                        continue;
                    }

                    ProducerRecord<String, byte[]> record = new ProducerRecord<>(route.kafkaTopic, payload);
                    RecordMetadata metadata = producer.send(record).get();
                    message.acknowledge();

                    if (metadata.offset() % 500 == 0) {
                        System.out.printf(
                            "Route %s forwarded offset=%d partition=%d%n",
                            route.label,
                            metadata.offset(),
                            metadata.partition()
                        );
                    }
                }
            } catch (Exception ex) {
                System.err.printf("Route %s failed: %s%n", route.label, ex.getMessage());
                sleep(RETRY_DELAY);
            } finally {
                closeQuietly(consumer);
                closeQuietly(session);
                closeQuietly(connection);
            }
        }
    }

    private static byte[] extractPayload(Message message) throws JMSException {
        if (message instanceof TextMessage) {
            String text = ((TextMessage) message).getText();
            return text == null ? new byte[0] : text.getBytes(StandardCharsets.UTF_8);
        }

        if (message instanceof BytesMessage) {
            BytesMessage bytesMessage = (BytesMessage) message;
            long length = bytesMessage.getBodyLength();
            if (length <= 0L) {
                return new byte[0];
            }
            if (length > Integer.MAX_VALUE) {
                throw new JMSException("message too large");
            }

            byte[] body = new byte[(int) length];
            int read = bytesMessage.readBytes(body);
            if (read < 0) {
                return new byte[0];
            }
            if (read < body.length) {
                byte[] trimmed = new byte[read];
                System.arraycopy(body, 0, trimmed, 0, read);
                return trimmed;
            }
            return body;
        }

        return Objects.toString(message, "").getBytes(StandardCharsets.UTF_8);
    }

    private static void closeQuietly(MessageConsumer consumer) {
        if (consumer == null) {
            return;
        }
        try {
            consumer.close();
        } catch (Exception ignored) {
        }
    }

    private static void closeQuietly(Session session) {
        if (session == null) {
            return;
        }
        try {
            session.close();
        } catch (Exception ignored) {
        }
    }

    private static void closeQuietly(Connection connection) {
        if (connection == null) {
            return;
        }
        try {
            connection.close();
        } catch (Exception ignored) {
        }
    }

    private static void sleep(Duration duration) {
        try {
            Thread.sleep(duration.toMillis());
        } catch (InterruptedException interruptedException) {
            Thread.currentThread().interrupt();
        }
    }

    private static String readEnv(String key, String fallback) {
        String value = System.getenv(key);
        if (value == null || value.isBlank()) {
            return fallback;
        }
        return value.trim();
    }

    private enum DestinationKind {
        QUEUE,
        TOPIC
    }

    private static final class RouteConfig {
        private final String label;
        private final String destinationName;
        private final DestinationKind kind;
        private final String kafkaTopic;

        private RouteConfig(String label, String destinationName, DestinationKind kind, String kafkaTopic) {
            this.label = label;
            this.destinationName = destinationName;
            this.kind = kind;
            this.kafkaTopic = kafkaTopic;
        }

        private static RouteConfig fromRawDestination(
            String label,
            String rawDestination,
            DestinationKind defaultKind,
            String kafkaTopic
        ) {
            String trimmed = rawDestination == null ? "" : rawDestination.trim();
            if (trimmed.startsWith("/queue/")) {
                return new RouteConfig(label, trimmed.substring("/queue/".length()), DestinationKind.QUEUE, kafkaTopic);
            }
            if (trimmed.startsWith("/topic/")) {
                return new RouteConfig(label, trimmed.substring("/topic/".length()), DestinationKind.TOPIC, kafkaTopic);
            }
            if (trimmed.startsWith("queue://")) {
                return new RouteConfig(label, trimmed.substring("queue://".length()), DestinationKind.QUEUE, kafkaTopic);
            }
            if (trimmed.startsWith("topic://")) {
                return new RouteConfig(label, trimmed.substring("topic://".length()), DestinationKind.TOPIC, kafkaTopic);
            }

            return new RouteConfig(label, trimmed, defaultKind, kafkaTopic);
        }
    }

    private static final class BridgeConfig {
        private final String activeMqBrokerUrl;
        private final String activeMqUser;
        private final String activeMqPassword;
        private final String kafkaBrokers;
        private final String kafkaClientId;

        private BridgeConfig(
            String activeMqBrokerUrl,
            String activeMqUser,
            String activeMqPassword,
            String kafkaBrokers,
            String kafkaClientId
        ) {
            this.activeMqBrokerUrl = activeMqBrokerUrl;
            this.activeMqUser = activeMqUser;
            this.activeMqPassword = activeMqPassword;
            this.kafkaBrokers = kafkaBrokers;
            this.kafkaClientId = kafkaClientId;
        }

        private static BridgeConfig fromEnv() {
            String host = readEnv("MQTT_IP_ADDRESS", "localhost");
            String port = readEnv("MQTT_PORT", "61616");
            String brokerUrl = readEnv("ACTIVEMQ_BROKER_URL", "tcp://" + host + ":" + port);

            return new BridgeConfig(
                brokerUrl,
                readEnv("MQTT_USER", ""),
                readEnv("MQTT_PASSWORD", ""),
                readEnv("KAFKA_BROKERS", "localhost:9092"),
                readEnv("KAFKA_CLIENT_ID", "itafm-amq-kafka-broker")
            );
        }

        private Properties kafkaProperties() {
            Properties properties = new Properties();
            properties.put(ProducerConfig.BOOTSTRAP_SERVERS_CONFIG, kafkaBrokers);
            properties.put(ProducerConfig.CLIENT_ID_CONFIG, kafkaClientId);
            properties.put(ProducerConfig.KEY_SERIALIZER_CLASS_CONFIG, StringSerializer.class.getName());
            properties.put(ProducerConfig.VALUE_SERIALIZER_CLASS_CONFIG, ByteArraySerializer.class.getName());
            properties.put(ProducerConfig.ACKS_CONFIG, "all");
            properties.put(ProducerConfig.ENABLE_IDEMPOTENCE_CONFIG, "true");
            properties.put(ProducerConfig.COMPRESSION_TYPE_CONFIG, "gzip");
            return properties;
        }
    }
}
