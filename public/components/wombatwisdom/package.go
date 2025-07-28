// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

// This package provides seamless integration between Wombat and wombatwisdom components
// with the ww_ prefix to avoid naming conflicts with existing Benthos components.
//
// The integration provides:
// - Native Benthos configuration interface
// - Full wombatwisdom functionality under the hood
// - Automatic bridging between Benthos and wombatwisdom data structures
// - Consistent error handling and logging
//
// Available components:
// - ww_nats: NATS messaging input/output using wombatwisdom NATS implementation
// - ww_demo: Demo component showing the integration pattern
//
// Additional wombatwisdom components can be easily added following the same pattern.
//
// Usage example:
//
//   input:
//     ww_nats:
//       url: nats://localhost:4222
//       subject: test.subject
//       queue: my-queue
//
//   output:
//     ww_nats:
//       url: nats://localhost:4222
//       subject: output.subject