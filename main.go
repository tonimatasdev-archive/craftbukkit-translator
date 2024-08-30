package main

import (
	"archive/zip"
	"bufio"
	"io"
	"net/http"
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
	println("Downloading CraftBukkit...")
	downloadNecessary()
	println("CraftBukkit downloaded correctly.")
	println("Unzipping CraftBukkit...")
	unzipCraftBukkit()
	println("CraftBukkit unzipped correctly.")
	println("Preparing CraftBukkit...")
	prepareCraftBukkit()
	println("CraftBukkit prepared correctly.")
	readBukkitSRG()
	println("Detected", len(bukkitClassesSRG), "class names and", len(bukkitDoubleClassesSRG), "double class names in the SRG.")
	loadBukkitJavaFiles()
	println("Detected " + strconv.Itoa(len(craftBukkitJavaFiles)) + " OldCraftBukkit classes.")
	processFiles()
}

func downloadNecessary() {
	err := os.MkdirAll("OldCraftBukkit", os.ModePerm)
	if err != nil {
		panic(err)
	}

	file, err := os.Create("./OldCraftBukkit/CraftBukkit.zip")
	if err != nil {
		panic(err)
	}
	defer fileError(file)

	resp, err := http.Get("https://hub.spigotmc.org/stash/rest/api/latest/projects/SPIGOT/repos/craftbukkit/archive?format=zip")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		panic(err)
	}
}

func unzipCraftBukkit() {
	src := "OldCraftBukkit/CraftBukkit.zip"
	dest := "OldCraftBukkit/"

	r, err := zip.OpenReader(src)
	if err != nil {
		panic(err)
	}
	defer func(r *zip.ReadCloser) {
		err := r.Close()

		if err != nil {
			panic(err)
		}
	}(r)

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {

			panic("illegal file path: " + fpath)
		}

		if f.FileInfo().IsDir() {
			err := os.MkdirAll(fpath, os.ModePerm)

			if err != nil {
				panic(err)
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			panic(err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())

		if err != nil {
			panic(err)
		}

		rc, err := f.Open()

		if err != nil {
			panic(err)
		}

		_, err = io.Copy(outFile, rc)

		if err != nil {
			panic(err)
		}

		fileError(outFile)
		err = rc.Close()

		if err != nil {
			panic(err)
		}
	}
}

func prepareCraftBukkit() {
	entries, err := os.ReadDir("OldCraftBukkit/")
	if err != nil {
		panic(err)
	}

	toDelete := []string{"OldCraftBukkit/src/assembly", "OldCraftBukkit/src/main/resources", "OldCraftBukkit/src/test",
		"OldCraftBukkit/nms-patches"}

	for _, entry := range entries {
		if !entry.IsDir() {
			toDelete = append(toDelete, "OldCraftBukkit/"+entry.Name())
		}
	}

	for _, path := range toDelete {
		err := os.RemoveAll(path)
		if err != nil {
			panic(err)
		}
	}
}

func readBukkitSRG() {
	file, err := os.Open("bukkit_srg.srg")

	if err != nil {
		println("Open file error:", err)
	}

	defer fileError(file)

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
		panic(err)
	}
}

func loadBukkitJavaFiles() {
	err := filepath.WalkDir("OldCraftBukkit", func(path string, d os.DirEntry, err error) error {

		if err != nil {
			panic(err)
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".java") {
			file, err := os.Open(path)

			if err != nil {
				panic(err)
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

			fileError(file)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}
}

func processFiles() {
	for x, craftBukkitFile := range craftBukkitJavaFiles {
		err := os.MkdirAll(strings.ReplaceAll(filepath.Dir(craftBukkitFile.Path), "Old", "New"), os.ModePerm)

		if err != nil {
			panic(err)
		}

		imports, codeImports, otherImports := getClassesToReplace(craftBukkitFile.Code)

		var newCode []string
		for _, line := range craftBukkitFile.Code {
			if strings.Contains(line, "import net.minecraft") {
				continue
			}

			newCode = append(newCode, translateLine(line, imports, codeImports, otherImports))
		}

		file, err := os.Create(strings.Replace(craftBukkitFile.Path, "Old", "New", 1))
		if err != nil {
			panic(err)
		}

		for _, line := range newCode {
			_, err := file.WriteString(line + "\n")
			if err != nil {
				panic(err)
			}
		}

		fileError(file)

		println("[" + strconv.Itoa(x+1) + "/" + strconv.Itoa(len(craftBukkitJavaFiles)) + "] " + file.Name())
	}
}

func getClassesToReplace(lines []string) ([]Class, []Class, []string) {
	var imports []Class
	var codeImports []Class
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

	return imports, codeImports, otherImports
}

func translateLine(line string, imports []Class, codeImports []Class, otherImports []string) string {
	if strings.Contains(line, "import") {
		return line
	}

	var allImports []Class

	allImports = append(allImports, imports...)
	allImports = append(allImports, codeImports...)

	for _, class := range allImports {
		toReplace := class.Import + class.OldClass

		for _, normalImport := range imports {
			if normalImport == class {
				toReplace = normalImport.OldClass
				break
			}
		}

		toReplaceSplit := strings.Split(toReplace, ".")

		if len(toReplaceSplit) == 2 {
			toReplace = toReplaceSplit[1]
		}

		if !strings.Contains(line, toReplace) {
			continue
		}

		var charNums []int
		charRightNow := 0

	continueWithLine:
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
					charRightNow = charNums[0] + newChars
					charNums = []int{}
					goto continueWithLine
				}
			} else {
				charNums = []int{}
			}
		}
	}

	for _, class := range bukkitDoubleClassesSRG {
		newClassSplit := strings.Split(class.Class, ".")
		oldClassSplit := strings.Split(class.OldClass, ".")

		charRightNow := -1
	next:
		if strings.Index(line, class.Import+newClassSplit[0]+"."+oldClassSplit[1]) > charRightNow {
			first := strings.Index(line, class.Import+newClassSplit[0]+"."+oldClassSplit[1])
			last := first + len(class.Import+newClassSplit[0]+"."+oldClassSplit[1]) - 1

			charNums := []int{first, last}

			if !unicode.IsLetter(rune(line[last+1])) {
				result, newChars := replace(line, charNums, class)
				line = result
				charRightNow = charNums[0] + newChars
				goto next
			}
		}
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

func fileError(file *os.File) {
	err := file.Close()
	if err != nil {
		panic(err)
	}
}
