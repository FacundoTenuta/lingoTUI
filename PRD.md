# lingoTUI — Product Requirements Document

## 1. Resumen

lingoTUI es una aplicación de terminal para macOS que ayuda a una persona hispanohablante a entender conversaciones, reuniones y audio del entorno cuando no domina inglés o alemán.

La app permite grabar audio desde el micrófono —y, en una etapa posterior, desde el audio del sistema—, transcribirlo con IA y generar resúmenes prácticos en español, inglés y alemán. También permite hacer consultas libres sobre traducciones, significados, respuestas sugeridas o contexto de lo escuchado.

El objetivo no es construir un simple traductor, sino un asistente lingüístico de supervivencia para vivir y trabajar en un país donde el usuario todavía no domina el idioma.

## 2. Contexto

El usuario tiene una oportunidad laboral en Viena, Austria, pero no sabe alemán y considera que su inglés es limitado. Esto genera una necesidad concreta: poder entender conversaciones laborales, reuniones, llamadas, trámites y situaciones cotidianas sin depender de una traducción literal frase por frase.

La herramienta debe priorizar comprensión, contexto y acción.

## 3. Problema

Las herramientas actuales de traducción y transcripción suelen resolver partes aisladas del problema:

- Traducen frases sueltas, pero no explican el contexto.
- Transcriben audio, pero no resumen lo importante.
- No combinan fácilmente micrófono y audio del sistema.
- No permiten consultar sobre lo que acaba de pasar.
- No están diseñadas para un flujo rápido desde terminal.

El usuario necesita saber:

- Qué dijeron.
- Qué significa.
- Qué debería responder.
- Qué vocabulario importante apareció.
- Cómo expresar una idea en alemán o inglés.

## 4. Objetivos

### Objetivo principal

Crear una TUI para macOS que capture audio, lo transcriba con IA y entregue resúmenes útiles en español, inglés y alemán.

### Objetivos secundarios

- Permitir consultas libres sobre idioma, traducción y contexto.
- Mantener contexto reciente de las transcripciones.
- Soportar providers de IA configurables por el usuario.
- Ofrecer una experiencia de conexión de providers similar a opencode.
- Diseñar la app de forma modular para poder agregar nuevos providers y fuentes de audio.

## 5. No objetivos

Para el MVP, lingoTUI no busca:

- Soportar Windows o Linux.
- Ser una app mobile.
- Tener UI gráfica.
- Ofrecer traducción simultánea perfecta en tiempo real.
- Identificar hablantes de forma avanzada.
- Funcionar completamente offline.
- Reemplazar aprendizaje formal de idiomas.

## 6. Usuario objetivo

### Usuario principal

Persona hispanohablante que se muda o trabaja en Austria/Alemania y necesita ayuda para entender alemán e inglés en contextos reales.

### Casos de uso

- Reuniones laborales.
- Entrevistas.
- Calls por Zoom, Meet o Teams.
- Conversaciones presenciales.
- Trámites.
- Consultas rápidas de vocabulario.
- Preparación de respuestas formales o informales.
- Revisión de lo que se dijo en una conversación reciente.

## 7. Plataforma inicial

El MVP soportará únicamente macOS.

Esta decisión reduce complejidad inicial, especialmente en captura de audio. La captura de micrófono será prioritaria. La captura de audio del sistema será considerada una funcionalidad avanzada porque en macOS puede requerir herramientas externas como BlackHole o Loopback.

## 8. Funcionalidades principales

### 8.1 Captura de audio desde micrófono

El usuario debe poder iniciar y detener una grabación desde la TUI.

Flujo esperado:

```text
> /record mic
Recording from microphone...

> /stop
Processing audio...
```

### 8.2 Captura de audio del sistema

La app debería soportar audio del sistema en una iteración posterior.

En macOS esto puede requerir configuración externa mediante un driver virtual de audio.

Ejemplos:

- BlackHole.
- Loopback.

Para el MVP, esta funcionalidad puede quedar marcada como experimental.

### 8.3 Transcripción con IA

La app debe convertir audio en texto usando un provider de IA configurado por el usuario.

Idiomas mínimos soportados:

- Español.
- Inglés.
- Alemán.

### 8.4 Resumen multilingüe

Luego de transcribir, la app debe generar un resumen en:

- Español.
- Inglés.
- Alemán.

El resumen debe priorizar comprensión y acción, no literalidad.

Ejemplo de salida:

```text
Summary — Spanish
La persona explicó que la reunión del viernes se mueve al lunes. También pidió revisar el contrato antes del mediodía.

Summary — English
The person explained that Friday's meeting is moved to Monday. They also asked you to review the contract before noon.

Zusammenfassung — Deutsch
Die Person erklärte, dass das Meeting vom Freitag auf Montag verschoben wird. Außerdem soll der Vertrag vor Mittag geprüft werden.
```

### 8.5 Consultas libres

El usuario debe poder hacer preguntas desde la TUI.

Ejemplos:

```text
> ¿Cómo se dice perro en alemán?
> ¿Qué significa "Kündigungsfrist"?
> ¿Cómo respondo educadamente que no entendí?
> Resumime qué me pidieron hacer.
> Dame una respuesta corta en alemán.
```

### 8.6 Contexto reciente

La app debe mantener contexto de la última transcripción o sesión para responder preguntas sobre lo escuchado.

Ejemplos:

```text
> ¿Qué fechas mencionaron?
> ¿Qué tareas me asignaron?
> ¿Tengo que responder algo?
```

### 8.7 Conexión de providers de IA

lingoTUI debe permitir conectar providers de IA usando una experiencia similar a opencode.

Flujo esperado:

```text
> /connect

Select provider:
  OpenAI
  OpenRouter
  Ollama
  Local
```

Para OpenAI:

```text
Auth method:
  Login with ChatGPT Plus/Pro
  Enter API key manually
```

La autenticación mediante ChatGPT Plus/Pro debe investigarse técnicamente. El ingreso manual de API key será el fallback obligatorio del MVP.

### 8.8 Selección de modelos

La app debe permitir elegir modelos para distintas tareas.

Ejemplo:

```text
> /models

Transcription model:
  gpt-4o-transcribe
  whisper-1

Chat model:
  gpt-4.1
  gpt-4o
```

## 9. MVP

El MVP debe validar el flujo principal con la menor complejidad posible.

### Incluido en MVP

- TUI básica para macOS.
- Grabación desde micrófono.
- Transcripción de audio.
- Resumen en español.
- Resumen opcional en inglés y alemán.
- Consultas libres.
- Uso del contexto de la última transcripción.
- Configuración de OpenAI mediante API key.
- Estructura interna preparada para múltiples providers.

### No incluido en MVP

- Captura robusta de audio del sistema.
- Login completo con ChatGPT Plus/Pro si requiere integración compleja.
- Soporte Windows/Linux.
- Traducción en tiempo real palabra por palabra.
- Modo offline.
- Identificación de hablantes.

## 10. Flujo principal del MVP

```text
Usuario abre lingoTUI
→ Conecta provider de IA si no existe configuración
→ Selecciona modelo de transcripción y chat
→ Inicia grabación desde micrófono
→ Detiene grabación
→ La app transcribe el audio
→ La app genera resumen
→ La app muestra resumen en español, inglés y alemán
→ Usuario hace preguntas sobre lo escuchado
→ La app responde usando el contexto reciente
```

## 11. Requisitos funcionales

### RF1 — Iniciar y detener grabación

El usuario debe poder iniciar y detener una grabación desde la TUI.

### RF2 — Capturar audio desde micrófono

El sistema debe poder grabar audio usando el micrófono de macOS.

### RF3 — Transcribir audio

El sistema debe enviar el audio a un provider de IA y obtener una transcripción.

### RF4 — Generar resumen

El sistema debe generar un resumen breve y accionable a partir de la transcripción.

### RF5 — Mostrar resumen multilingüe

El sistema debe mostrar el resumen en español, inglés y alemán.

### RF6 — Permitir consultas libres

El usuario debe poder escribir preguntas y recibir respuestas generadas por IA.

### RF7 — Usar contexto reciente

Las respuestas deben poder usar la última transcripción y resumen como contexto.

### RF8 — Conectar provider de IA

El usuario debe poder configurar un provider de IA desde la TUI mediante un comando tipo `/connect`.

### RF9 — Guardar credenciales localmente

La app debe guardar credenciales localmente de forma segura o, como mínimo para MVP, en una ubicación documentada y excluida de versionado.

Ubicación sugerida:

```text
~/Library/Application Support/lingotui/auth.json
```

### RF10 — Seleccionar modelos

El usuario debe poder seleccionar modelos para transcripción y chat/resumen.

## 12. Requisitos no funcionales

- La app debe ser rápida de usar desde terminal.
- Debe funcionar inicialmente en macOS.
- Debe dejar claro cuándo está grabando.
- Debe manejar errores de permisos de micrófono.
- Debe manejar errores de provider/API sin crashear.
- Debe evitar guardar audio o transcripciones por defecto salvo que el usuario lo active.
- Debe permitir cambiar provider/modelo sin reescribir la lógica principal.
- Debe separar captura de audio, transcripción, resumen, consultas y presentación TUI.

## 13. Privacidad

La app trabaja con audio potencialmente sensible. Por eso debe seguir estas reglas:

- Mostrar claramente cuándo está grabando.
- No grabar sin acción explícita del usuario.
- No guardar audio por defecto.
- No guardar transcripciones por defecto.
- Permitir borrar contexto reciente.
- Documentar qué datos se envían al provider de IA.

## 14. Arquitectura conceptual

```text
TUI
  ↓
Audio Capture
  ↓
Transcription Provider
  ↓
Summarization Service
  ↓
Context Store
  ↓
Query Engine
  ↓
TUI Output
```

Componentes sugeridos:

- `AudioCapture`: graba audio desde micrófono o sistema.
- `TranscriptionProvider`: convierte audio en texto.
- `SummaryService`: genera resúmenes multilingües.
- `ProviderRegistry`: registra providers disponibles.
- `ProviderAuth`: maneja conexión/autenticación.
- `CredentialStore`: guarda credenciales locales.
- `ModelRegistry`: lista modelos disponibles.
- `ContextStore`: mantiene contexto reciente.
- `TUI`: maneja interacción con el usuario.

## 15. Providers de IA

### Provider inicial

OpenAI será el provider inicial recomendado.

### Métodos de autenticación deseados

Para OpenAI:

1. Login con ChatGPT Plus/Pro, si es técnicamente viable y permitido.
2. API key manual como fallback obligatorio.

### Providers futuros

- OpenRouter.
- Ollama.
- Anthropic.
- Google Gemini.
- Groq.
- Azure OpenAI.
- Providers OpenAI-compatible.

## 16. Riesgos técnicos

### RT1 — Captura de audio del sistema en macOS

Capturar audio de parlantes en macOS puede requerir un driver virtual. Esta funcionalidad no debe bloquear el MVP.

### RT2 — Login con ChatGPT Plus/Pro

El login con cuenta ChatGPT Plus/Pro puede no tener una API pública estable para apps externas. Debe investigarse antes de comprometerlo como parte del MVP.

### RT3 — Latencia

El procesamiento puede tardar dependiendo del tamaño del audio y del provider. Para el MVP se prioriza grabación por bloques, no tiempo real estricto.

### RT4 — Costos

Transcripción, resumen y consultas pueden generar costos. La app debe dejar claro qué provider/modelo se está usando.

### RT5 — Privacidad y consentimiento

Grabar conversaciones puede tener implicancias legales y éticas. La app debe ser explícita sobre cuándo graba y qué envía a terceros.

## 17. Métricas de éxito

El MVP será exitoso si:

- El usuario puede grabar audio desde el micrófono en macOS.
- La app genera una transcripción útil.
- La app resume lo importante en español.
- La app puede mostrar resumen en inglés y alemán.
- El usuario puede hacer preguntas sobre lo escuchado.
- El flujo completo funciona sin configuración compleja después de conectar el provider.
- Una conversación breve puede procesarse en menos de 30 segundos después de detener la grabación.

## 18. Iteraciones sugeridas

### Iteración 1 — MVP micrófono

- TUI básica.
- `/connect` con OpenAI API key.
- `/models` básico.
- Grabación de micrófono.
- Transcripción.
- Resumen en español/inglés/alemán.
- Consultas sobre contexto reciente.

### Iteración 2 — Audio del sistema experimental

- Soporte documentado para BlackHole o Loopback.
- Selección de fuente de audio.
- Grabación desde audio del sistema.

### Iteración 3 — Experiencia avanzada

- Login tipo ChatGPT Plus/Pro si es viable.
- Múltiples providers.
- Mejor manejo de historial.
- Plantillas de respuestas formales/informales.
- Modo aprendizaje de vocabulario.

## 19. Preguntas abiertas

- ¿Qué stack se usará para construir la TUI?
- ¿Qué modelo de transcripción será el default?
- ¿Se guardará historial opcionalmente?
- ¿Cómo se manejarán credenciales de forma segura en macOS?
- ¿Qué tan viable es replicar el login ChatGPT Plus/Pro de opencode?
- ¿La app debe funcionar durante reuniones en vivo o alcanza con grabaciones por bloques?
