# Informe Funcional - Go Fiber LiveView

## Resumen de Funcionalidades

Go Fiber LiveView proporciona un framework completo para desarrollar aplicaciones web interactivas en tiempo real. El sistema permite crear componentes reactivos usando únicamente Go, sin necesidad de escribir JavaScript manual.

## Funcionalidades del Sistema

### 1. Sistema de Componentes

#### 1.1 Componentes Base
**Descripción**: Sistema de componentes reutilizables con estado y ciclo de vida.

**Características**:
- **Definición declarativa** mediante interfaces
- **Estado encapsulado** en structs de Go
- **Templates HTML** integrados
- **Eventos personalizables** con callbacks

**Ejemplo de uso**:
```go
type Reloj struct {
    Hora time.Time
}

func (r *Reloj) GetTemplate() string {
    return `<div>{{.Hora.Format("15:04:05")}}</div>`
}

func (r *Reloj) Start() {
    r.Hora = time.Now()
}
```

#### 1.2 Componentes Predefinidos

**Clock (Reloj)**:
- Actualización automática cada 60ms
- Formato de hora personalizable
- Sincronización entre múltiples clientes

**Button (Botón)**:
- Eventos click manejados server-side
- Estados visuales dinámicos
- Callbacks personalizados

**Input (Campo de entrada)**:
- Validación en tiempo real
- Sincronización bidireccional
- Eventos de cambio instantáneos

### 2. Sistema de Layouts

#### 2.1 Gestión de Layouts
**Descripción**: Sistema para organizar y coordinar múltiples componentes en una página.

**Funcionalidades**:
- **Mounting de componentes** en templates HTML
- **Broadcasting de mensajes** entre componentes
- **Parsing automático** de elementos HTML
- **Gestión de estado** centralizada

**Sintaxis**:
```html
<div>{{mount "componente1"}}</div>
<div>{{mount "componente2"}}</div>
```

#### 2.2 Comunicación Entre Componentes
- **Mensajes directos** entre componentes específicos
- **Broadcasting** a todos los componentes
- **Event bubbling** desde componentes hijos
- **State sharing** mediante layouts

### 3. Comunicación WebSocket

#### 3.1 Conexión Automática
**Descripción**: Establecimiento automático de conexión WebSocket entre cliente y servidor.

**Características**:
- **Conexión transparente** sin configuración manual
- **Reconexión automática** en caso de pérdidas
- **Heartbeat** para mantener conexión activa
- **Cleanup automático** de recursos

#### 3.2 Sincronización de Estado
- **Estado servidor → cliente**: Actualizaciones automáticas del DOM
- **Eventos cliente → servidor**: Captura de interacciones del usuario
- **Bidireccionalidad**: Flujo de datos en ambas direcciones
- **Optimización**: Solo se envían cambios incrementales

### 4. Integración WebAssembly

#### 4.1 Cliente WASM
**Descripción**: Código Go compilado a WebAssembly para ejecutar en el navegador.

**Funcionalidades**:
- **Manipulación DOM** directa desde Go
- **Event handling** del navegador
- **Serialización JSON** automática
- **Interop JavaScript** cuando sea necesario

#### 4.2 Operaciones DOM Disponibles
```go
// Modificación de contenido
driver.SetHTML("<h1>Nuevo contenido</h1>")
driver.SetText("Texto plano")
driver.SetValue("Valor del input")

// Modificación de estilos
driver.SetStyle("color: red; font-size: 16px")

// Obtención de valores
html := driver.GetHTML()
text := driver.GetText()
value := driver.GetValue()
```

### 5. Sistema de Eventos

#### 5.1 Eventos del Cliente
**Descripción**: Captura y manejo de eventos del navegador desde el servidor.

**Eventos soportados**:
- **Click**: Clicks en elementos
- **Input**: Cambios en campos de texto
- **Submit**: Envío de formularios
- **Custom**: Eventos personalizados

**Definición de eventos**:
```go
func (c *MiComponente) GetDriver() view.LiveDriver {
    return view.NewComponentDriver(c).
        Event("click", func() {
            c.Contador++
        }).
        Event("reset", func() {
            c.Contador = 0
        })
}
```

#### 5.2 Eventos del Servidor
- **Timers**: Eventos programados temporalmente
- **Triggers**: Eventos disparados por cambios de estado
- **Lifecycle**: Eventos del ciclo de vida de componentes

### 6. Sistema de Templates

#### 6.1 Templates HTML
**Descripción**: Sistema de plantillas HTML con sintaxis de Go templates.

**Características**:
- **Sintaxis familiar** de Go templates
- **Binding automático** de datos
- **Funciones auxiliares** predefinidas
- **Composición** de templates

#### 6.2 Funciones de Template Disponibles
```html
<!-- Montar componentes -->
{{mount "nombre_componente"}}

<!-- Acceso a datos del componente -->
{{.Campo}}
{{.Metodo}}

<!-- Condicionales -->
{{if .Condicion}}contenido{{end}}

<!-- Loops -->
{{range .Lista}}{{.}}{{end}}
```

## Interacción Entre Funcionalidades

### Flujo Completo de una Interacción

1. **Usuario interactúa** con elemento en el navegador
2. **WASM captura evento** del DOM
3. **WebSocket envía** evento al servidor
4. **Componente procesa** evento y actualiza estado
5. **Template se re-renderiza** con nuevo estado
6. **Diferencias se calculan** entre estado anterior y nuevo
7. **WebSocket envía** actualizaciones al cliente
8. **WASM aplica cambios** al DOM
9. **Interfaz se actualiza** visualmente

### Ejemplo de Flujo: Aplicación TODO

#### Escenario: Agregar nueva tarea

1. **Usuario escribe** en campo de texto
   - Event: `input` → Servidor actualiza estado temporal

2. **Usuario hace click** en botón "Agregar"
   - Event: `click` → Servidor valida y guarda tarea

3. **Lista se actualiza** automáticamente
   - Servidor re-renderiza lista de tareas
   - Cambios se envían a todos los clientes conectados

4. **Persistencia** (en el ejemplo)
   - Datos se guardan en archivo JSON
   - Sincronización con otros usuarios

## Casos de Uso Implementados

### 1. Aplicación de Reloj (Example1)
**Funcionalidades demostradas**:
- Componente simple con auto-actualización
- Timer server-side de alta frecuencia
- Sincronización de tiempo entre clientes

### 2. Sistema de Chat (Example2)
**Funcionalidades demostradas**:
- Comunicación en tiempo real
- Salas públicas y privadas
- Gestión de usuarios conectados
- Broadcasting selectivo de mensajes

### 3. Aplicación TODO (Example_todo)
**Funcionalidades demostradas**:
- CRUD completo (Create, Read, Update, Delete)
- Persistencia en archivo JSON
- Sincronización multi-cliente
- Validación de datos
- Estados de tareas (pendiente/completada)

## APIs Públicas Disponibles

### API de Componentes
```go
type Component interface {
    GetTemplate() string    // Define template HTML
    Start()                // Inicialización del componente
    GetDriver() LiveDriver // Obtiene driver para operaciones
}
```

### API de LiveDriver
```go
type LiveDriver interface {
    // Modificación DOM
    SetHTML(string)
    SetText(string)
    SetValue(interface{})
    SetStyle(string)
    
    // Consulta DOM
    GetHTML() string
    GetText() string
    GetValue() string
    
    // Eventos
    ExecuteEvent(name string, data interface{})
    
    // Lifecycle
    Mount(component Component) LiveDriver
    Commit()
}
```

### API de PageControl
```go
type PageControl struct {
    Title  string           // Título de la página
    Path   string           // Ruta URL
    Router *fiber.App       // Router de Fiber
}

func (pc *PageControl) Register(factory func() LiveDriver)
```

## Limitaciones Funcionales Actuales

### 1. Persistencia
- **Solo archivos JSON** en ejemplos
- **No hay base de datos** integrada
- **Concurrencia limitada** en escrituras
- **Sin transacciones** atómicas

### 2. Validación
- **Validación básica** en ejemplos
- **Sin sistema de validación** robusto
- **Sin mensajes de error** estructurados
- **Sin validación client-side**

### 3. Routing
- **Routing simple** con Fiber
- **Sin nested routes**
- **Sin parámetros dinámicos** en URLs
- **Sin navegación SPA**

### 4. Estado
- **Estado local** por componente
- **Sin estado global** compartido
- **Sin persistencia** automática de estado
- **Sin time-travel debugging**

## Extensiones Posibles

### 1. Componentes Adicionales
- **DataTable**: Tablas con paginación y filtros
- **Form**: Formularios con validación automática
- **Modal**: Ventanas modales
- **Chart**: Gráficos y visualizaciones

### 2. Funcionalidades Avanzadas
- **Routing SPA**: Navegación sin recarga
- **State management**: Estado global compartido
- **File upload**: Subida de archivos
- **Real-time collaboration**: Colaboración en tiempo real

### 3. Integraciones
- **Database ORM**: Integración con bases de datos
- **Authentication**: Sistema de autenticación
- **Caching**: Sistema de caché
- **Monitoring**: Métricas y observabilidad

## Conclusiones Funcionales

### Fortalezas
- **Simplicidad**: Desarrollo únicamente con Go
- **Tiempo real**: Actualizaciones automáticas
- **Componentes**: Arquitectura modular y reutilizable
- **Performance**: Aprovecha la velocidad de Go

### Oportunidades de Mejora
- **Funcionalidades web avanzadas**: Routing, formularios, validación
- **Persistencia robusta**: Integración con bases de datos
- **Herramientas de desarrollo**: Debugging, hot reload
- **Documentación**: Más ejemplos y guías

### Idoneidad del Framework
**Ideal para**:
- Dashboards en tiempo real
- Aplicaciones colaborativas
- Sistemas de monitoreo
- Prototipos rápidos

**No recomendado para**:
- Aplicaciones con SEO crítico
- Sistemas con alta carga de tráfico
- Aplicaciones móviles nativas
- Sistemas que requieren JavaScript complejo