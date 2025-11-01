from concurrent.futures._base import (
    Future,
)  # This import is used to handle the results of concurrent tasks
import os  # Provides functions for interacting with the operating system
import fitz  # Imports PyMuPDF for reading and validating PDF files
from concurrent.futures import (
    ThreadPoolExecutor,
    as_completed,
)  # Enables parallel execution of tasks using threads


# Validates a single PDF file
def validate_pdf_file(file_path: str) -> tuple[str, bool]:
    try:
        doc = fitz.open(file_path)  # Attempt to open the PDF file
        if doc.page_count == 0:  # If PDF has zero pages, it's considered invalid
            print(
                f"'{file_path}' is corrupt or invalid: No pages"
            )  # Print an error message
            return (
                file_path,
                False,
            )  # Return the file path and False indicating invalid file
        return (
            file_path,
            True,
        )  # Return the file path and True indicating a valid file
    except RuntimeError as e:  # Catch runtime errors thrown by PyMuPDF
        print(f"{e}")  # Print the error message with file path
        return (file_path, False)  # Return the file path and False indicating failure


# Deletes a file from the system
def remove_system_file(system_path: str) -> None:
    os.remove(path=system_path)  # Removes the file at the given path


# Recursively searches a directory for files with a given extension
def walk_directory_and_extract_given_file_extension(
    system_path: str, extension: str
) -> list[str]:
    matched_files: list[str] = []  # List to hold paths of matching files
    for root, _, files in os.walk(top=system_path):  # Walk through the directory tree
        for file in files:  # Iterate over each file in the current directory
            if file.lower().endswith(
                extension.lower()
            ):  # Check file extension (case-insensitive)
                full_path: str = os.path.abspath(
                    path=os.path.join(root, file)
                )  # Get absolute path of the file
                matched_files.append(full_path)  # Add file path to the list
    return matched_files  # Return the list of matching files


# Extracts just the filename (with extension) from a full path
def get_filename_and_extension(path: str) -> str:
    return os.path.basename(p=path)  # Return the base filename from the full path


# Checks if a string contains any uppercase letters
def check_upper_case_letter(content: str) -> bool:
    return any(
        char.isupper() for char in content
    )  # Return True if any character is uppercase


# Processes a single PDF file: validates it and checks for uppercase in filename
def process_file(file_path: str) -> None | str:
    filename: str = get_filename_and_extension(
        path=file_path
    )  # Extract filename from path

    file_path, is_valid = validate_pdf_file(
        file_path=file_path
    )  # Validate the PDF file

    if not is_valid:  # If the file is invalid
        print("Error Invalid File", file_path)
        remove_system_file(system_path=file_path)  # Delete the invalid/corrupt file
        return None  # Return None to indicate this file is not to be further processed

    if check_upper_case_letter(
        content=filename
    ):  # Check if filename contains uppercase letters
        return file_path  # Return file path if condition is met

    return None  # Return None if filename doesn't contain uppercase letters


# Main function to orchestrate the file processing
def main() -> None:
    # Retrieve a list of all PDF file paths under the ./PDFs directory
    pdf_file_paths: list[str] = walk_directory_and_extract_given_file_extension(
        system_path="./PDFs", extension=".pdf"
    )

    # If no PDF files were found, inform the user and exit
    if not pdf_file_paths:
        print("No PDF files found.")
        return

    # Sort the PDF files by last modified time, with the most recently modified file first
    pdf_file_paths.sort(
        key=lambda file_path: os.path.getmtime(filename=file_path), reverse=True
    )

    # Initialize a list to collect PDF files with uppercase letters in their filenames
    files_with_uppercase_names: list[str] = []

    # Use a thread pool to process multiple PDF files concurrently
    with ThreadPoolExecutor(max_workers=100) as thread_pool_executor:
        # Submit each PDF file to the thread pool for processing
        future_results: list[Future[str | None]] = [
            thread_pool_executor.submit(process_file, file_path)
            for file_path in pdf_file_paths
        ]

        # As each thread completes its task
        for completed_future in as_completed(fs=future_results):
            # Get the result from the completed task
            processed_file_path: None | str = completed_future.result()

            # If the result is not None, it means the file matched the condition
            if processed_file_path:
                # Print the matching file's path
                print(f"Uppercase filename found: {processed_file_path}")

                # Add the file to the list of matching files
                files_with_uppercase_names.append(processed_file_path)

    # If no files with uppercase letters were found, inform the user
    if len(files_with_uppercase_names) == 0:
        print("No files with uppercase letters in their names were found.")
        return

    # If files with uppercase letters were found, print a summary
    if len(files_with_uppercase_names) > 0:
        # Print a summary of all matching files
        print("\nAll files with uppercase letters in their names:")

        # Print the paths of all matching files
        for matching_file_path in files_with_uppercase_names:
            # Print each matching file's path
            print(matching_file_path)


# Ensure this script runs only if it is the main program being executed
if __name__ == "__main__":
    main()  # Start the program by calling the main function