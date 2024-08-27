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
	bukkitSRG            = make(map[string]Class)
	craftBukkitJavaFiles []File
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
	log.Println("Detected " + strconv.Itoa(len(bukkitSRG)) + " class names in the SRG.")
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

				bukkitSRG[oldClassName] = Class{
					OldClass: oldClassName,
					Import:   strings.ReplaceAll(strings.ReplaceAll(lineSplit[0], oldSplitSplit[len(oldSplitSplit)-1], ""), "/", "."),
					Class:    className,
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

		toReplace, toCodeReplace := getStaticClassesToReplace(craftbukkitFile.Code)

		var newCode []string
		for _, line := range craftbukkitFile.Code {
			if strings.Contains(line, "import net.minecraft") {
				continue
			}

			newCode = append(newCode, translateLine(line, toReplace, toCodeReplace))
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

func getStaticClassesToReplace(lines []string) ([]string, []Class) {
	var imports []string
	var codeImports []Class

	for _, line := range lines {
		if strings.Contains(line, "import") && strings.Contains(line, "net.minecraft") {
			importWithoutImport := strings.ReplaceAll(line, "import ", "")
			imports = append(imports, strings.ReplaceAll(importWithoutImport, ";", ""))
		}

		for _, class := range bukkitSRG {
			if strings.Contains(line, class.Import+class.OldClass) {
				addImport := true
				for _, importClass := range codeImports {
					if importClass == class {
						addImport = false
						break
					}
				}

				if addImport {
					codeImports = append(codeImports, class)
				}
			}
		}
	}

	return imports, codeImports
}

func translateLine(line string, toReplace []string, toCodeReplace []Class) string {
	if strings.Contains(line, "import") {
		return line
	}

	for _, oldCodeClassImport := range toCodeReplace {
		line = strings.ReplaceAll(line, oldCodeClassImport.Import+oldCodeClassImport.OldClass, oldCodeClassImport.Import+oldCodeClassImport.Class)
	}

	for _, oldClassImport := range toReplace {
		oldClassImportSplit := strings.Split(oldClassImport, ".")
		doubleClassImport := bukkitSRG[oldClassImportSplit[len(oldClassImportSplit)-2]+"."+oldClassImportSplit[len(oldClassImportSplit)-1]]

		if !unicode.IsUpper(rune(oldClassImportSplit[len(oldClassImportSplit)-2][0])) {
			doubleClassImport = Class{}
		}

		oldClassName := oldClassImportSplit[len(oldClassImportSplit)-1]

		class := bukkitSRG[oldClassName]
		if strings.Contains(line, class.OldClass) {
			var charNums []int
		label:
			for char := range line {
				charNums = append(charNums, char)

				if strings.HasPrefix(oldClassName, createWord(line, charNums)) {
					if oldClassName == createWord(line, charNums) && checkBothBounds(line, charNums, class.Class, class.Import) {
						if !haveOtherImport(line, charNums[0]) {
							isDouble, doubleClass := isDoubleClass(line, charNums[len(charNums)-1], class.OldClass)

							if isDouble {
								line = strings.ReplaceAll(line, doubleClass.OldClass, class.Import+doubleClass.Class)
								charNums = []int{}
								doubleClassImport = Class{}
								goto label
							} else {
								line = replace(line, charNums, class.Class, class.Import, doubleClassImport)
								charNums = []int{}
								doubleClassImport = Class{}
								goto label
							}
						} else {
							charNums = []int{}
						}
					}
				} else {
					charNums = []int{}
				}
			}
		}
	}

	return line
}

func replace(str string, charNums []int, replace string, importOnly string, doubleClass Class) string {
	minimum := charNums[0]
	maximum := charNums[len(charNums)-1]

	isDoubleClassCheck := false
	if doubleClass.Class != "" {
		replace = doubleClass.Class
		isDoubleClassCheck = true
	}

	var result string
	for i := 0; i < len(str)+1; i++ {
		if i >= minimum && i <= maximum {

		} else {
			if maximum+1 == i {
				if isDoubleClassCheck {
					importOnly = doubleClass.Import
				}

				if haveImport(str, charNums[0], importOnly) {
					result += replace
				} else {
					result += importOnly + replace
				}
			}

			if i <= len(str)-1 {
				result += string(str[i])
			}
		}
	}

	return result
}

func checkBothBounds(str string, charNums []int, replace string, importOnly string) bool {
	resultAfter := false

	if len(str)-1 <= charNums[len(charNums)-1] {
		resultAfter = true
	} else {
		if !unicode.IsLetter(rune(str[charNums[len(charNums)-1]+1])) {
			resultAfter = true
		}
	}

	resultBefore := false

	if charNums[0] == 0 {
		resultBefore = true
	} else {
		if !unicode.IsLetter(rune(str[charNums[0]-1])) {
			if createWord(str, charNums) != replace || !haveImport(str, charNums[0], importOnly) {
				resultBefore = true
			}
		}
	}

	return resultAfter && resultBefore
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

func isDoubleClass(str string, lastChar int, replace string) (bool, Class) {
	if len(str)-1 <= lastChar {
		return false, Class{}
	}

	var charNums []int
	if string(str[lastChar+1]) == "." {
		for i := lastChar + 2; i < len(str); i++ {
			if unicode.IsLetter(rune(str[i])) {
				charNums = append(charNums, i)
			} else {
				break
			}
		}
	}

	exits := bukkitSRG[replace+"."+createWord(str, charNums)]
	if exits.Class != "" {
		return true, exits
	}

	return false, Class{}

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
	if strings.HasPrefix(createWord(str, charNums), "org.bukkit") || strings.HasPrefix(createWord(str, charNums), "com.mojang") {
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

func createWord(str string, charNums []int) string {
	result := ""

	for _, x := range charNums {
		result += string(str[x])
	}

	return result
}
