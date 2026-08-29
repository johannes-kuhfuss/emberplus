# emberplus

`emberplus` is a Go consumer library for the Ember+ control protocol. The current implementation targets Glow DTD 2.50 and supports both S101 framing variants.

The project originated from the Zabbix Ember+ implementation and retains its AGPL-3.0 license.

## Supported protocol features

- Signed 64-bit Ember integers, binary BER REAL values, booleans, strings, octets, NULL, and multi-byte RELATIVE-OIDs.
- Definite and indefinite BER containers, long-form lengths, and unknown Glow content fields.
- Nodes, parameters, matrices, functions, templates, commands, qualified root elements, streams, and invocation results.
- Matrix targets, sources, connections, labels, enum maps, stream descriptors, schemas, and template references.
- GetDirectory, parameter changes, matrix connection changes, Subscribe/Unsubscribe, and function invocation.
- Escaped S101 framing with CRC validation and the Glow 2.50 non-escaping framing variant.
- Multipart reassembly across arbitrary TCP read boundaries and mandatory keep-alive responses while receiving.

## Basic use

```go
client, err := emberclient.NewEmberClient("console.example", 9000)
if err != nil {
    return err
}
if err := client.Connect(); err != nil {
    return err
}
defer client.Disconnect()

tree, err := client.GetRootCollection()
if err != nil {
    return err
}

parameter, err := tree.GetElementByPath("1.128.3")
```

All potentially blocking operations also have context-aware forms. Context cancellation interrupts a blocked network read or write even when the client timeout is disabled.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := client.SetParameter(ctx, "1.128.3", -12.5)
result, err := client.Invoke(ctx, "1.4.1", 51, int64(1), int64(2))
```

Matrix operations use the exported Glow constants such as `ConnectionAbsolute`, `ConnectionConnect`, and `ConnectionDisconnect`.

```go
response, err := client.SetMatrixConnections(ctx, "1.20", []ember.MatrixConnection{
    {Target: 2, Sources: []int{7}, Operation: ember.ConnectionAbsolute},
})
```

## Notifications and keep-alives

Each connected client has one permanent read pump. It reassembles S101 messages, answers keep-alives, matches request responses, and routes unrelated Glow roots to the notification API. `Serve`, `ReceiveRootContext`, and request methods can therefore run concurrently on one connection. `Serve` uses the supplied context for its lifetime and does not stop merely because the client's per-operation timeout elapses.

```go
err := client.Serve(ctx, func(message ember.RootMessage) error {
    // Apply message.Elements updates or process message.Streams.
    return nil
})
```

Only one notification consumer should use `Serve`, `ReceiveRoot`, or `Receive` at a time; those APIs share one bounded notification queue. Under overload the oldest queued notification is discarded to preserve recency. `LatestElement` retains the newest top-level element update by path even if a notification event is discarded.

For monitoring, enumerate the required directory once, run `Serve` continuously, and expose the resulting latest state to the metrics collector. Repeated directory polling is unnecessary because GetDirectory establishes update traffic for ordinary Ember elements.

Optional diagnostics report only actionable pump conditions: messages skipped while matching a response, notification overflow, and terminal read-pump errors.

```go
client.SetDiagnosticHandler(func(event emberclient.Diagnostic) {
    log.Printf("Ember diagnostic: kind=%s path=%s skipped=%d dropped=%d err=%v",
        event.Kind, event.Path, event.SkippedRoots, event.DroppedNotifications, event.Err)
})
```

## Element decoding APIs

`ElementCollection.Populate` and the existing `GetElementCollection` methods retain the original element/value representation for existing callers. New code should use `DecodeRoot`, `ElementCollection.PopulateGlow250`, or `GetElementCollectionGlow250`; these preserve signed `int64` values and expose the complete Glow 2.50 model.

Prefer `NewElementCollection` in new code.

The protocol definition and reference implementations are available from [Lawo/ember-plus](https://github.com/Lawo/ember-plus).
