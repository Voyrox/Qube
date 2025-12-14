#!/bin/bash

set -e

usage() {
    echo "Usage: $0 [OPTIONS] [FILE...]"
    echo "Convert .tar.gz files to .tar files"
    echo ""
    echo "Options:"
    echo "  -h, --help           Show this help message"
    echo "  -r, --remove         Remove original .tar.gz files after conversion"
    echo "  -d, --directory DIR  Convert all .tar.gz files in a directory"
    echo ""
    echo "Examples:"
    echo "  $0 file.tar.gz                    # Convert file.tar.gz to file.tar"
    echo "  $0 -r file1.tar.gz file2.tar.gz  # Convert and remove originals"
    echo "  $0 -d /path/to/dir                # Convert all .tar.gz in directory"
    exit 1
}

REMOVE_ORIGINAL=false
DIRECTORY=""
FILES=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            usage
            ;;
        -r|--remove)
            REMOVE_ORIGINAL=true
            shift
            ;;
        -d|--directory)
            DIRECTORY="$2"
            shift 2
            ;;
        *)
            FILES+=("$1")
            shift
            ;;
    esac
done

convert_file() {
    local input_file="$1"
    local output_file="${input_file%.tar.gz}.tar"

    if [[ ! -f "$input_file" ]]; then
        echo "Error: File not found: $input_file"
        return 1
    fi

    if [[ ! "$input_file" =~ \.tar\.gz$ ]]; then
        echo "Error: File is not a .tar.gz file: $input_file"
        return 1
    fi

    if [[ -f "$output_file" ]]; then
        echo "Warning: Output file already exists, skipping: $output_file"
        return 0
    fi

    echo "Converting: $input_file -> $output_file"
    
    local temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" RETURN
    
    if ! tar -xzf "$input_file" -C "$temp_dir"; then
        echo "  ✗ Failed to extract: $input_file"
        return 1
    fi
    
    if ! tar -cf "$output_file" -C "$temp_dir" .; then
        echo "  ✗ Failed to create tar archive: $output_file"
        rm -f "$output_file"
        return 1
    fi

    local original_size=$(du -h "$input_file" | cut -f1)
    local new_size=$(du -h "$output_file" | cut -f1)
    echo "  ✓ Success! Original: $original_size, Uncompressed: $new_size"

    if [[ "$REMOVE_ORIGINAL" == true ]]; then
        rm -f "$input_file"
        echo "  ✓ Removed original: $input_file"
    fi
}

if [[ -n "$DIRECTORY" ]]; then
    if [[ ! -d "$DIRECTORY" ]]; then
        echo "Error: Directory not found: $DIRECTORY"
        exit 1
    fi

    echo "Searching for .tar.gz files in: $DIRECTORY"
    while IFS= read -r -d '' file; do
        convert_file "$file"
    done < <(find "$DIRECTORY" -maxdepth 1 -name "*.tar.gz" -print0)
fi

if [[ ${#FILES[@]} -gt 0 ]]; then
    for file in "${FILES[@]}"; do
        convert_file "$file"
    done
fi

if [[ -z "$DIRECTORY" && ${#FILES[@]} -eq 0 ]]; then
    usage
fi

echo "Conversion complete!"
