package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

var (
	bukkitClassesSRG       = make(map[string]Class)
	bukkitDoubleClassesSRG = make(map[string]Class)
	craftBukkitJavaFiles   []File
)

// Download it https://hub.spigotmc.org/stash/rest/api/latest/projects/SPIGOT/repos/craftbukkit/archive?format=zip

type Class struct {
	OldClass string
	Import   string
	Class    string
}

type File struct {
	Code []string
	Path string
}

func main() {
	readBukkitSRG()
	log.Println("Detected", len(bukkitClassesSRG), "class names and", len(bukkitDoubleClassesSRG), "double class names in the SRG.")
	loadBukkitJavaFiles()
	log.Println("Detected " + strconv.Itoa(len(craftBukkitJavaFiles)) + " OldCraftBukkit classes.")

	processFiles()
}

func readBukkitSRG() {
	file, err := os.Open("bukkit_srg.srg")

	if err != nil {
		log.Fatalln("Open file error:", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "\t") {
			lineSplit := strings.Split(line, " ")

			if len(lineSplit) == 2 {
				oldSplitSplit := strings.Split(lineSplit[0], "/")
				oldClassName := strings.ReplaceAll(oldSplitSplit[len(oldSplitSplit)-1], "$", ".")

				splitSplit := strings.Split(lineSplit[1], "/")
				className := strings.ReplaceAll(splitSplit[len(splitSplit)-1], "$", ".")

				if len(strings.Split(className, ".")) == 2 {
					bukkitDoubleClassesSRG[oldClassName] = Class{
						OldClass: oldClassName,
						Import:   strings.ReplaceAll(strings.ReplaceAll(lineSplit[0], oldSplitSplit[len(oldSplitSplit)-1], ""), "/", "."),
						Class:    className,
					}
				} else {
					bukkitClassesSRG[oldClassName] = Class{
						OldClass: oldClassName,
						Import:   strings.ReplaceAll(strings.ReplaceAll(lineSplit[0], oldSplitSplit[len(oldSplitSplit)-1], ""), "/", "."),
						Class:    className,
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln("Error durante la lectura del archivo:", err)
	}
}

func loadBukkitJavaFiles() {
	err := filepath.WalkDir("OldCraftBukkit", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".java") {
			file, err := os.Open(path)

			if err != nil {
				log.Fatalln("Open file error:", err)
			}

			scanner := bufio.NewScanner(file)

			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			craftBukkitJavaFiles = append(craftBukkitJavaFiles, File{
				Code: lines,
				Path: path,
			})
			file.Close()
		}
		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}

func processFiles() {
	for x, craftbukkitFile := range craftBukkitJavaFiles {
		err := os.MkdirAll(strings.ReplaceAll(filepath.Dir(craftbukkitFile.Path), "Old", "New"), 0777)

		if err != nil {
			log.Fatalln("MkdirAll error:", err)
		}

		imports, otherImports := getClassesToReplace(craftbukkitFile.Code)

		var newCode []string
		for _, line := range craftbukkitFile.Code {
			if strings.Contains(line, "import net.minecraft") {
				continue
			}

			newCode = append(newCode, translateLine(line, imports, otherImports))
		}

		file, err := os.Create(strings.Replace(craftbukkitFile.Path, "Old", "New", 1))
		if err != nil {
			fmt.Println("Error creating the file:", err)
			return
		}

		for _, line := range newCode {
			_, err := file.WriteString(line + "\n")
			if err != nil {
				fmt.Println("Error writing in the file:", err)
				return
			}
		}

		file.Close()

		log.Println("[" + strconv.Itoa(x+1) + "/" + strconv.Itoa(len(craftBukkitJavaFiles)) + "] " + file.Name())
	}
}

func getClassesToReplace(lines []string) ([]Class, []string) {
	var imports []Class
	var otherImports []string

	for _, line := range lines {
		if strings.Contains(line, "import") && strings.Contains(line, "net.minecraft") {
			importWithoutImport := strings.ReplaceAll(strings.ReplaceAll(line, "import ", ""), ";", "")

			importSplit := strings.Split(importWithoutImport, ".")
			classImport := bukkitClassesSRG[importSplit[len(importSplit)-1]]
			doubleClassImport := bukkitDoubleClassesSRG[importSplit[len(importSplit)-2]+"."+importSplit[len(importSplit)-1]]

			if doubleClassImport.Class == "" {
				imports = append(imports, classImport)
			} else {
				imports = append(imports, doubleClassImport)
			}

			continue
		}

		if strings.Contains(line, "import") && !strings.Contains(line, "net.minecraft") {
			importWithoutImport := strings.ReplaceAll(strings.ReplaceAll(line, "import ", ""), ";", "")
			importSplit := strings.Split(importWithoutImport, ".")
			otherImports = append(otherImports, importSplit[len(importSplit)-1])
			continue
		}

		for _, class := range bukkitClassesSRG {
			if strings.Contains(line, class.Import+class.OldClass) {
				addImport := true
				for _, importClass := range imports {
					if importClass == class {
						addImport = false
						break
					}
				}

				if addImport {
					imports = append(imports, class)
				}
			}
		}
	}

	return imports, otherImports
}

func translateLine(line string, imports []Class, otherImports []string) string {
	if strings.Contains(line, "import") {
		return line
	}

	for _, class := range imports {
		toReplace := class.OldClass
		toReplaceSplit := strings.Split(toReplace, ".")

		if len(toReplaceSplit) == 2 {
			toReplace = toReplaceSplit[1]
		}

		if !strings.Contains(line, toReplace) {
			continue
		}

		var charNums []int
		charRightNow := 0

	label:
		for char := charRightNow; char < len(line); char++ {
			charNums = append(charNums, char)

			str := createStr(line, charNums)

			if strings.HasPrefix(toReplace, str) {
				if str != toReplace {
					continue
				}

				if isIt(line, charNums, str, class, otherImports) {
					result, newChars := replace(line, charNums, class)
					line = result
					charRightNow += newChars
					charNums = []int{}
					goto label
				}
			} else {
				charNums = []int{}
			}
		}
	}

	for _, class := range bukkitDoubleClassesSRG {
		newClassSplit := strings.Split(class.Class, ".")
		oldClassSplit := strings.Split(class.OldClass, ".")

		line = strings.ReplaceAll(line, class.Import+newClassSplit[0]+"."+oldClassSplit[1], class.Import+newClassSplit[0]+"."+newClassSplit[1])
	}

	// Manual fixes
	line = strings.ReplaceAll(line, "net.minecraft.world.net.minecraft.world.inventory.AbstractContainerMenu", "net.minecraft.world.inventory.AbstractContainerMenu")

	return line
}

func replace(str string, charNums []int, class Class) (string, int) {
	minimum := charNums[0]
	maximum := charNums[len(charNums)-1]

	var result string
	charsAdded := 0
	for i := 0; i < len(str)+1; i++ {
		if i >= minimum && i <= maximum {

		} else {
			if maximum+1 == i {
				if haveImport(str, charNums[0], class.Import) {
					result += class.Class
					charsAdded += len(class.Class)
				} else {
					result += class.Import + class.Class
					charsAdded += len(class.Import + class.Class)
				}
			}

			if i <= len(str)-1 {
				result += string(str[i])
			}
		}
	}

	return result, charsAdded
}

func isIt(line string, charNums []int, str string, class Class, otherImports []string) bool {
	resultAfter := false
	resultBefore := false
	isNotOther := true

	if len(line)-1 <= charNums[len(charNums)-1] {
		resultAfter = true
	} else {
		if !unicode.IsLetter(rune(line[charNums[len(charNums)-1]+1])) {
			resultAfter = true
		}
	}

	if charNums[0] == 0 {
		resultBefore = true
	} else {
		if !unicode.IsLetter(rune(line[charNums[0]-1])) {
			resultBefore = true
		}
	}

	if !haveImport(line, charNums[0], class.Import) {
		for _, other := range otherImports {
			if str == other {
				isNotOther = false
				break
			}
		}
	}

	if haveOtherImport(line, charNums[0]) {
		isNotOther = false
	}

	return resultAfter && resultBefore && isNotOther
}

func haveImport(str string, charNum int, importOnly string) bool {
	firstImportChar := charNum - len(importOnly)

	if firstImportChar < 0 {
		return false
	}

	var resultStr string
	for i := firstImportChar; i < charNum; i++ {
		resultStr += string(str[i])
	}

	return importOnly == resultStr
}

func haveOtherImport(str string, firstChar int) bool {
	if firstChar == 0 {
		return false
	}

	var charNums []int
	if string(str[firstChar-1]) == "." {
		for i := firstChar; i > 0; i-- {
			if string(str[i]) == "." || unicode.IsLetter(rune(str[i])) {
				charNums = append(charNums, i)
			} else {
				break
			}
		}
	}

	reverseArray(charNums)
	createdStr := createStr(str, charNums)
	if strings.HasPrefix(createdStr, "org.bukkit") || strings.HasPrefix(createdStr, "com.mojang") {
		return true
	}

	return false
}

func reverseArray(arr []int) {
	n := len(arr)
	for i := 0; i < n/2; i++ {
		arr[i], arr[n-i-1] = arr[n-i-1], arr[i]
	}
}

func createStr(str string, charNums []int) string {
	result := ""

	for _, x := range charNums {
		result += string(str[x])
	}

	return result
}
