#!/usr/bin/env bash
set -eo pipefail

echo "Generating Dafny code..."

DAFNY_VERSION=$(dafny --version)
echo "Using Dafny version: $DAFNY_VERSION"

# Translate to Go
dafny translate go verification/dafny/Reducer.dfy || true
dafny translate go verification/dafny/EffectReducer.dfy || true

# Check if the output directory was created
if [ ! -d "verification/dafny/Reducer-go/src/Reducer" ]; then
    echo "Dafny translation failed to generate the Go files."
    exit 1
fi

# Copy to the internal package
echo "Copying generated code to internal/dafny_reducer..."
mkdir -p internal/dafny_reducer
cp verification/dafny/Reducer-go/src/Reducer/Reducer.go internal/dafny_reducer/Reducer.go
if [ -d "verification/dafny/EffectReducer-go/src/EffectReducer" ]; then
    cp verification/dafny/EffectReducer-go/src/EffectReducer/EffectReducer.go internal/dafny_reducer/EffectReducer.go
fi

# Fix import paths
echo "Fixing Dafny runtime import paths..."
sed -i '' 's|"dafny"|"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"|g' internal/dafny_reducer/Reducer.go
sed -i '' 's|"System_"|"github.com/dafny-lang/DafnyRuntimeGo/v4/System_"|g' internal/dafny_reducer/Reducer.go
if [ -f "internal/dafny_reducer/EffectReducer.go" ]; then
    sed -i '' 's|"dafny"|"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"|g' internal/dafny_reducer/EffectReducer.go
    sed -i '' 's|"System_"|"github.com/dafny-lang/DafnyRuntimeGo/v4/System_"|g' internal/dafny_reducer/EffectReducer.go
fi

# Cleanup
rm -rf verification/dafny/Reducer-go
rm -rf verification/dafny/EffectReducer-go

echo "Dafny code generation complete."
