package com.aerothai.itafm.broker;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Properties;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;

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

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

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

        Map<String, RouteStats> routeStats = new LinkedHashMap<>();
        for (RouteConfig route : routes) {
            routeStats.put(route.label, new RouteStats(route));
        }

        HttpServer monitorServer = startMonitorServer(config, routeStats);
        KafkaProducer<String, byte[]> producer = new KafkaProducer<>(config.kafkaProperties());
        ExecutorService executor = Executors.newFixedThreadPool(routes.size());

        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            running = false;
            System.out.println("Shutting down ActiveMQ -> Kafka broker...");
            if (monitorServer != null) {
                monitorServer.stop(1);
            }
            executor.shutdownNow();
            producer.flush();
            producer.close(Duration.ofSeconds(10));
        }));

        for (RouteConfig route : routes) {
            RouteStats stats = routeStats.get(route.label);
            executor.submit(() -> consumeForever(config, stats, producer));
        }

        try {
            executor.awaitTermination(Long.MAX_VALUE, TimeUnit.DAYS);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
    }

    private static void consumeForever(BridgeConfig config, RouteStats routeStats, KafkaProducer<String, byte[]> producer) {
        RouteConfig route = routeStats.route;
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
                routeStats.markOnline();

                while (running && !Thread.currentThread().isInterrupted()) {
                    Message message = consumer.receive(1000L);
                    if (message == null) {
                        continue;
                    }
                    routeStats.recordReceived();

                    byte[] payload = extractPayload(message);
                    if (payload.length == 0) {
                        routeStats.recordEmptyPayload();
                        message.acknowledge();
                        continue;
                    }

                    ProducerRecord<String, byte[]> record = new ProducerRecord<>(route.kafkaTopic, payload);
                    RecordMetadata metadata = producer.send(record).get();
                    routeStats.recordForward(metadata, payload.length);
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
                routeStats.recordError(ex);
                System.err.printf("Route %s failed: %s%n", route.label, ex.getMessage());
                sleep(RETRY_DELAY);
            } finally {
                closeQuietly(consumer);
                closeQuietly(session);
                closeQuietly(connection);
                routeStats.markOffline();
            }
        }
    }

    private static HttpServer startMonitorServer(BridgeConfig config, Map<String, RouteStats> routeStats) {
        if (!config.monitorEnabled) {
            return null;
        }

        try {
            HttpServer server = HttpServer.create(new InetSocketAddress(config.monitorHost, config.monitorPort), 0);
            server.createContext("/", exchange -> writeText(exchange, 200,
                "ActiveMQ Kafka broker monitor\n\nGET /health\nGET /routes\n"));
            server.createContext("/health", exchange -> writeJson(exchange, 200, renderHealth(routeStats)));
            server.createContext("/routes", exchange -> writeJson(exchange, 200, renderRoutes(routeStats)));
            server.start();
            System.out.printf("Monitor online at http://%s:%d%n", config.monitorHost, config.monitorPort);
            return server;
        } catch (IOException ex) {
            System.err.printf(
                "Monitor server failed to start at %s:%d: %s%n",
                config.monitorHost,
                config.monitorPort,
                ex.getMessage()
            );
            return null;
        }
    }

    private static String renderHealth(Map<String, RouteStats> routeStats) {
        long onlineCount = 0L;
        for (RouteStats stats : routeStats.values()) {
            if (stats.online) {
                onlineCount += 1L;
            }
        }

        return new StringBuilder(128)
            .append("{\"status\":\"ok\",\"running\":")
            .append(running)
            .append(",\"route_count\":")
            .append(routeStats.size())
            .append(",\"online_routes\":")
            .append(onlineCount)
            .append(",\"timestamp\":\"")
            .append(Instant.now())
            .append("\"}")
            .toString();
    }

    private static String renderRoutes(Map<String, RouteStats> routeStats) {
        StringBuilder json = new StringBuilder(2048);
        json.append("{\"running\":")
            .append(running)
            .append(",\"timestamp\":\"")
            .append(Instant.now())
            .append("\",\"routes\":[");

        boolean first = true;
        for (RouteStats stats : routeStats.values()) {
            if (!first) {
                json.append(',');
            }
            first = false;
            stats.appendJson(json);
        }
        json.append("]}");
        return json.toString();
    }

    private static void writeJson(HttpExchange exchange, int status, String body) throws IOException {
        if (!"GET".equalsIgnoreCase(exchange.getRequestMethod())) {
            writeText(exchange, 405, "method not allowed\n");
            return;
        }

        byte[] payload = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json; charset=utf-8");
        exchange.sendResponseHeaders(status, payload.length);
        try (OutputStream responseBody = exchange.getResponseBody()) {
            responseBody.write(payload);
        }
    }

    private static void writeText(HttpExchange exchange, int status, String body) throws IOException {
        if (!"GET".equalsIgnoreCase(exchange.getRequestMethod())) {
            status = 405;
            body = "method not allowed\n";
        }

        byte[] payload = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "text/plain; charset=utf-8");
        exchange.sendResponseHeaders(status, payload.length);
        try (OutputStream responseBody = exchange.getResponseBody()) {
            responseBody.write(payload);
        }
    }

    private static String quoteJson(String value) {
        if (value == null) {
            return "";
        }

        StringBuilder escaped = new StringBuilder(value.length() + 16);
        for (int i = 0; i < value.length(); i++) {
            char ch = value.charAt(i);
            switch (ch) {
                case '\\':
                case '"':
                    escaped.append('\\').append(ch);
                    break;
                case '\b':
                    escaped.append("\\b");
                    break;
                case '\f':
                    escaped.append("\\f");
                    break;
                case '\n':
                    escaped.append("\\n");
                    break;
                case '\r':
                    escaped.append("\\r");
                    break;
                case '\t':
                    escaped.append("\\t");
                    break;
                default:
                    if (ch < 0x20) {
                        escaped.append(String.format("\\u%04x", (int) ch));
                    } else {
                        escaped.append(ch);
                    }
            }
        }
        return escaped.toString();
    }

    private static String formatEpoch(long value) {
        if (value <= 0L) {
            return "";
        }
        return Instant.ofEpochMilli(value).toString();
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

    private static int readIntEnv(String key, int fallback) {
        String value = System.getenv(key);
        if (value == null || value.isBlank()) {
            return fallback;
        }
        try {
            return Integer.parseInt(value.trim());
        } catch (NumberFormatException ex) {
            return fallback;
        }
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

    private static final class RouteStats {
        private final RouteConfig route;
        private final long startedAt = System.currentTimeMillis();
        private final AtomicLong receivedCount = new AtomicLong(0L);
        private final AtomicLong forwardedCount = new AtomicLong(0L);
        private final AtomicLong emptyPayloadCount = new AtomicLong(0L);
        private final AtomicLong errorCount = new AtomicLong(0L);
        private final AtomicLong totalForwardedBytes = new AtomicLong(0L);
        private final AtomicLong lastReceivedAt = new AtomicLong(0L);
        private final AtomicLong lastForwardedAt = new AtomicLong(0L);
        private final AtomicLong lastOffset = new AtomicLong(-1L);
        private final AtomicLong lastPartition = new AtomicLong(-1L);

        private volatile boolean online = false;
        private volatile String lastError = "";

        private RouteStats(RouteConfig route) {
            this.route = route;
        }

        private void markOnline() {
            this.online = true;
        }

        private void markOffline() {
            this.online = false;
        }

        private void recordReceived() {
            receivedCount.incrementAndGet();
            lastReceivedAt.set(System.currentTimeMillis());
        }

        private void recordEmptyPayload() {
            emptyPayloadCount.incrementAndGet();
        }

        private void recordForward(RecordMetadata metadata, int payloadBytes) {
            forwardedCount.incrementAndGet();
            totalForwardedBytes.addAndGet(payloadBytes);
            lastForwardedAt.set(System.currentTimeMillis());
            lastOffset.set(metadata.offset());
            lastPartition.set(metadata.partition());
            lastError = "";
        }

        private void recordError(Exception exception) {
            errorCount.incrementAndGet();
            lastError = exception == null ? "" : Objects.toString(exception.getMessage(), "");
        }

        private void appendJson(StringBuilder json) {
            json.append("{\"label\":\"")
                .append(quoteJson(route.label))
                .append("\",\"active_mq\":\"")
                .append(quoteJson(route.kind.name().toLowerCase() + ":" + route.destinationName))
                .append("\",\"kafka_topic\":\"")
                .append(quoteJson(route.kafkaTopic))
                .append("\",\"online\":")
                .append(online)
                .append(",\"received_count\":")
                .append(receivedCount.get())
                .append(",\"forwarded_count\":")
                .append(forwardedCount.get())
                .append(",\"empty_payload_count\":")
                .append(emptyPayloadCount.get())
                .append(",\"error_count\":")
                .append(errorCount.get())
                .append(",\"forwarded_bytes\":")
                .append(totalForwardedBytes.get())
                .append(",\"last_offset\":")
                .append(lastOffset.get())
                .append(",\"last_partition\":")
                .append(lastPartition.get())
                .append(",\"started_at\":\"")
                .append(formatEpoch(startedAt))
                .append("\",\"last_received_at\":\"")
                .append(formatEpoch(lastReceivedAt.get()))
                .append("\",\"last_forwarded_at\":\"")
                .append(formatEpoch(lastForwardedAt.get()))
                .append("\",\"last_error\":\"")
                .append(quoteJson(lastError))
                .append("\"}");
        }
    }

    private static final class BridgeConfig {
        private final String activeMqBrokerUrl;
        private final String activeMqUser;
        private final String activeMqPassword;
        private final String kafkaBrokers;
        private final String kafkaClientId;
        private final boolean monitorEnabled;
        private final String monitorHost;
        private final int monitorPort;

        private BridgeConfig(
            String activeMqBrokerUrl,
            String activeMqUser,
            String activeMqPassword,
            String kafkaBrokers,
            String kafkaClientId,
            boolean monitorEnabled,
            String monitorHost,
            int monitorPort
        ) {
            this.activeMqBrokerUrl = activeMqBrokerUrl;
            this.activeMqUser = activeMqUser;
            this.activeMqPassword = activeMqPassword;
            this.kafkaBrokers = kafkaBrokers;
            this.kafkaClientId = kafkaClientId;
            this.monitorEnabled = monitorEnabled;
            this.monitorHost = monitorHost;
            this.monitorPort = monitorPort;
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
                readEnv("KAFKA_CLIENT_ID", "itafm-amq-kafka-broker"),
                Boolean.parseBoolean(readEnv("MONITOR_ENABLED", "true")),
                readEnv("MONITOR_HOST", "0.0.0.0"),
                readIntEnv("MONITOR_PORT", 18080)
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
