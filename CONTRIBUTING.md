# Contributing to Fault-Tolerant Distributed Caching

👍 Thank you for considering contributing to our distributed caching system!

## 🚀 Getting Started

### Prerequisites
- Go 1.21 or higher
- Docker & Docker Compose
- Git

### Development Setup

1. **Fork the repository**
2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR_USERNAME/Fault-Tolerant-Distributed-Caching-A-Scalable-Approach-Through-Consistent-Hashing.git
   cd Fault-Tolerant-Distributed-Caching-A-Scalable-Approach-Through-Consistent-Hashing
   ```

3. **Install dependencies**:
   ```bash
   go mod tidy
   ```

4. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## 📝 Code Style Guidelines

- Follow Go best practices and conventions
- Use `gofmt` and `golint` for code formatting
- Add unit tests for new functionality
- Update documentation for API changes

## 🧪 Testing

Before submitting a PR:

```bash
# Run unit tests
go test ./...

# Run integration tests
docker-compose up --build

# Run load tests
cd load_test && ./run_test.sh
```

## 📋 Pull Request Process

1. **Update documentation** if needed
2. **Add tests** for new features
3. **Ensure all tests pass**
4. **Update the README** if you change functionality
5. **Submit your PR** with a clear title and description

## 🐛 Bug Reports

When filing an issue, make sure to answer these questions:

- What version of Go are you using?
- What operating system are you using?
- What did you do?
- What did you expect to see?
- What did you see instead?

## 💡 Feature Requests

We welcome feature requests! Please:

- Check if the feature already exists
- Describe the feature clearly
- Explain why it would be useful
- Consider implementation approach

## 📜 Code of Conduct

Please be respectful and constructive in all interactions.

## 📞 Questions?

Feel free to open an issue for questions or reach out via email.
