# Go Fiber LiveView

Un framework para crear aplicaciones web interactivas en tiempo real usando Go, inspirado en Phoenix LiveView. Combina la potencia del backend de Go con actualizaciones dinámicas del frontend mediante WebSockets y WebAssembly.

## ¿Qué hace este proyecto?

Go Fiber LiveView permite desarrollar aplicaciones web completamente interactivas escribiendo únicamente código Go. Las actualizaciones de la interfaz se manejan automáticamente mediante WebSockets, y la manipulación del DOM se realiza a través de WebAssembly compilado desde Go.

### Características principales:

- **Componentes reactivos**: Crea componentes reutilizables con estado
- **Actualizaciones en tiempo real**: Sin necesidad de JavaScript manual
- **WebSockets automáticos**: Comunicación bidireccional transparente
- **WebAssembly**: Manipulación DOM directa desde Go
- **Integración con Fiber**: Aprovecha el rendimiento de Fiber v2

## Instalación y configuración

### Prerrequisitos

- Go 1.23.4 o superior
- Un navegador web moderno con soporte para WebAssembly

### Instalación

1. **Clonar el repositorio**:
```bash
git clone https://github.com/arturoeanton/go-fiber-live-view.git
cd go-fiber-live-view
```

2. **Construir el módulo WebAssembly**:
```bash
./build_wasm.sh
```

3. **Instalar dependencias**:
```bash
cd liveview
go mod tidy
```

## Uso básico

### Ejemplo 1: Reloj en tiempo real

```bash
cd example/example1
go run main.go
```

Visita `http://localhost:3000` para ver un reloj que se actualiza automáticamente.

### Ejemplo 2: Chat en tiempo real

```bash
cd example/example2
go run main.go
```

Aplicación de chat con salas públicas y privadas.

### Ejemplo 3: Aplicación TODO

```bash
cd example/example_todo
go run main.go
```

CRUD completo con persistencia y sincronización en tiempo real.

## Crear tu primer componente

### 1. Definir el componente

```go
package main

import (
    "github.com/arturoeliasanton/go-fiber-live-view/liveview/view"
    "github.com/gofiber/fiber/v2"
)

type MiComponente struct {
    Contador int
}

func (c *MiComponente) GetTemplate() string {
    return `
    <div>
        <h2>Contador: {{.Contador}}</h2>
        <button onclick="increment()">Incrementar</button>
    </div>
    `
}

func (c *MiComponente) Start() {
    c.Contador = 0
}

func (c *MiComponente) GetDriver() view.LiveDriver {
    return view.NewComponentDriver(c).
        Event("increment", func() {
            c.Contador++
        })
}
```

### 2. Registrar el componente

```go
func main() {
    app := fiber.New()
    
    home := view.PageControl{
        Title:  "Mi App",
        Path:   "/",
        Router: app,
    }
    
    home.Register(func() view.LiveDriver {
        view.New("contador", &MiComponente{})
        return view.NewLayout("layout", `
            <div>{{mount "contador"}}</div>
        `)
    })
    
    app.Listen(":3000")
}
```

## Estructura del proyecto

```
/
├── liveview/           # Framework principal
│   ├── view/          # Core del sistema LiveView
│   ├── components/    # Componentes reutilizables
│   └── assets/        # Archivos WASM y JS generados
├── wasm/              # Código WebAssembly del cliente
├── example/           # Ejemplos de uso
│   ├── example1/      # Reloj básico
│   ├── example2/      # Chat en tiempo real
│   ├── example3/      # Ejemplo simple
│   └── example_todo/  # Aplicación TODO
└── build_wasm.sh      # Script de construcción WASM
```

## Dependencias principales

- **Fiber v2**: Framework web de alto rendimiento
- **WebSocket**: Comunicación en tiempo real
- **UUID**: Generación de identificadores únicos
- **Template**: Sistema de plantillas de Go

## Contribuir al proyecto

### Estilo de código

- Seguir las convenciones de Go (`go fmt`, `go vet`)
- Documentar funciones públicas con comentarios
- Usar nombres descriptivos para variables y funciones
- Mantener funciones pequeñas y enfocadas

### Estructura de Pull Requests

1. **Fork** el repositorio
2. **Crear branch** descriptivo: `feature/nueva-funcionalidad`
3. **Commits** atómicos con mensajes claros
4. **Tests** para nuevas funcionalidades
5. **Documentación** actualizada si es necesario

### Proceso de contribución

1. **Reportar issue** antes de grandes cambios
2. **Discutir** la implementación propuesta
3. **Implementar** siguiendo el estilo del proyecto
4. **Testing** exhaustivo
5. **Code review** collaborative

### Áreas que necesitan contribución

- **Tests unitarios**: El proyecto necesita cobertura de testing
- **Documentación**: Más ejemplos y guías
- **Seguridad**: Revisión y hardening de seguridad
- **Performance**: Optimizaciones y benchmarks
- **Componentes**: Biblioteca de componentes comunes

## Roadmap

- **v0.2**: Sistema de testing y CI/CD
- **v0.3**: Mejoras de seguridad y validación
- **v0.4**: Biblioteca de componentes estándar
- **v1.0**: Release estable para producción

## Licencia

Este proyecto está licenciado bajo la Licencia BSD de 3 cláusulas. Ver el archivo `LICENSE` para más detalles.

## Soporte

- **Issues**: Reporta bugs o solicita features
- **Discussions**: Preguntas y ayuda general
- **Wiki**: Documentación adicional y tutoriales

## Reconocimientos

Inspirado en Phoenix LiveView del ecosistema Elixir, adaptado para aprovechar la potencia y simplicidad de Go.