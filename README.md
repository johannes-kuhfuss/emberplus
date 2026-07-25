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

`ReceiveRootContext` reads unsolicited parameter updates, stream collections, and invocation results. `Serve` runs this receive loop continuously and therefore also answers peer keep-alive requests while the application is otherwise idle.
`Serve` uses the supplied context for its lifetime and does not stop merely because the client's per-operation timeout elapses.

```go
err := client.Serve(ctx, func(message ember.RootMessage) error {
    // Apply message.Elements updates or process message.Streams.
    return nil
})
```

Use one goroutine as the connection owner. The request methods serialize themselves, but `Serve` must not run concurrently with request methods because ordinary Ember directory and value-change responses do not carry correlation identifiers. Concurrent receive attempts return `emberclient.ErrReceiveActive` instead of consuming each other's messages. Applications that need continuous notifications and request/response operations at the same time should use separate connections.

## Compatibility API

`ElementCollection.Populate` and the existing `GetElementCollection` methods retain the original element/value representation for existing callers. New code should use `DecodeRoot`, `ElementCollection.PopulateGlow250`, or `GetElementCollectionGlow250`; these preserve signed `int64` values and expose the complete Glow 2.50 model.

`NewElementConnection` remains available as a compatibility alias. Prefer `NewElementCollection` in new code.

The protocol definition and reference implementations are available from [Lawo/ember-plus](https://github.com/Lawo/ember-plus).
