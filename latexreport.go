package latexreport

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
)

var (
	alphabet = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K",
		"L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}
	NCPU = 1 // Number of cpus to use
)

type Report struct {
	FileName   string
	StandAlone bool // standalone figure or full report?
	txt        string
	Outdir     string
	PNGconvert bool // convert report to PNG?
}

func NewReport(FileName string) (r *Report) {
	r = new(Report)
	r.FileName = FileName
	r.StandAlone = false
	r.Outdir = ""
	r.PNGconvert = false
	r.txt = `
\documentclass[8pt]{extarticle}
\usepackage{fontspec}
\setmainfont{Helvetica}
\usepackage[controls,loop]{animate}
\usepackage{graphicx}
\usepackage{caption}
\usepackage[left=0.7in,right=0.7in,top=0.7in,bottom=0.7in]{geometry}
\usepackage{float}
\setlength{\tabcolsep}{0pt}
\begin{document}
`
	return
}

func NewStandAloneReport(FileName string, x, y float64) (r *Report) {
	r = new(Report)
	r.FileName = FileName
	r.StandAlone = true
	r.txt = `
\documentclass[8pt]{extarticle}
\usepackage{fontspec}
\setmainfont[Path=/home/marshall/tessumcm/.fonts/,UprightFont=*]{Helvetica}
\usepackage[controls,loop]{animate}
\usepackage{graphicx}
\usepackage{caption}`
	r.txt += fmt.Sprintf("\\usepackage[noheadfoot,nomarginpar,margin=0pt,paperwidth=%fin,paperheight=%fin]{geometry}", x, y)
	r.txt += `
\pagestyle{empty}
\parindent=0pt
\usepackage{float}
\setlength{\tabcolsep}{0pt}
\begin{document}
`
	return
}

func numrows(numfigs, numcols int) (nrows, remainder int) {
	mod := math.Mod(float64(numfigs), float64(numcols))
	if mod == 0 {
		nrows = numfigs / numcols
		remainder = 0
	} else {
		nrows = numfigs/numcols + 1
		remainder = int(mod)
	}
	return
}

func (r *Report) MapFigure(mapFileNames []string, Titles []string,
	legendFiles []string, caption string, ncolumns int) {
	r.txt += "\n\\begin{figure}[H]\n\\begin{center}\n" +
		fmt.Sprintf("\\begin{tabular}{%v}\n", strings.Repeat("c", ncolumns))

	nrows, remainder := numrows(len(mapFileNames), ncolumns)
	i := 0
	ii := 0
	for j := 0; j < nrows; j++ {
		var endofrow int
		if j == nrows-1 && remainder != 0 {
			endofrow = i + remainder
		} else {
			endofrow = i + ncolumns
		}
		for {
			if i == endofrow {
				break
			}
			r.txt += fmt.Sprintf("\\includegraphics[width=%f\\textwidth]{%s}",
				1./float64(ncolumns), mapFileNames[i])
			if i < endofrow-1 {
				r.txt += " &\n"
			}
			i++
		}
		r.txt += " \\\\\n"
		for {
			if ii == endofrow {
				break
			}
			r.txt += fmt.Sprintf("(%s) %s", alphabet[ii], Titles[ii])
			if ii < endofrow-1 {
				r.txt += " &\n"
			}
			ii++
		}
		r.txt += " \\\\\n"
	}
	r.txt += "\\end{tabular}\n"
	for _, legendFile := range legendFiles {
		r.txt += fmt.Sprintf("\\includegraphics[width=0.49\\textwidth]{%s}\n",
			legendFile)
	}
	r.txt += "\\\\\n"
	if r.StandAlone {
		r.txt += fmt.Sprintf("\\end{center}\n\\caption*{%s}\n\\end{figure}\n",
			caption)
	} else {
		r.txt += fmt.Sprintf("\\end{center}\n\\caption{%s}\n\\end{figure}\n",
			caption)
	}
	return
}

func (r *Report) Animation(filebase string, numFrames int, caption string,
	framesPerSec int) {
	r.txt += "\\begin{figure}[H]\n" +
		fmt.Sprintf("\\animategraphics[width=\\textwidth]{%d}{%s}{0000}{%04i}\n",
			framesPerSec, filebase, numFrames-1) +
		fmt.Sprintf("\\caption{%s}\n\\end{figure}\n", caption)
	return
}

func (r *Report) Plot(filename string, caption string) {
	r.txt += fmt.Sprintf("\\begin{figure}[H]\n\\includegraphics{%s}\n",
		filename) +
		fmt.Sprintf("\\caption{%s}\n\\end{figure}\n", caption)
	return
}

func (r *Report) Write() {
	r.txt += "\n\\end{document}\n"
	f, err := os.Create(r.FileName + ".tex")
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(f, r.txt)
	if err != nil {
		panic(err)
	}
	f.Close()
	cmd := new(exec.Cmd)
	if r.Outdir == "" {
		cmd = exec.Command("xelatex", r.FileName+".tex")
	} else {
		cmd = exec.Command("xelatex",
			fmt.Sprintf("-output-directory=%s", r.Outdir),
			r.FileName+".tex")
	}
	out, err := cmd.CombinedOutput()
	output := fmt.Sprintf("%s", out)
	if strings.Index(output, "Emergency") != -1 {
		panic(output)
	}
	if r.PNGconvert == true {
		r.ConvertToPng()
	}
	return
}

func (r *Report) ConvertToPng() {
	cmd := exec.Command("convert", "-density", "400",
		r.FileName+".pdf", r.FileName+".png")
	out, err := cmd.CombinedOutput()
	output := fmt.Sprintf("%s", out)
	if err != nil {
		panic(fmt.Errorf(output))
	}
}

func ReportServer(reportchan chan *Report, finished chan int) {
	queue := make(chan *Report)
	finished2 := make(chan int)
	// spawn workers
	for i := 0; i < NCPU; i++ {
		go reportworker(i, queue, finished2)
	}
	// Sent reports to worker
	for {
		report := <-reportchan
		if report == nil {
			break
		}
		queue <- report
	}
	// all work is done
	// signal workers there is no more work
	for n := 0; n < NCPU; n++ {
		queue <- nil
	}
	for n := 0; n < NCPU; n++ {
		<-finished2
	}
	finished <- 0
}

func reportworker(id int, queue chan *Report, finished chan int) {
	var report *Report
	for {
		// get work item from the queue
		report = <-queue
		if report == nil {
			break
		}
		report.Write()
	}
	finished <- 0
}

func CreateVideo(filenamePattern, outfile string) {
	cmd := exec.Command("ffmpeg", "-y", "-f", "image2", "-r", "15", "-i",
		filenamePattern, outfile)
	out, err := cmd.CombinedOutput()
	output := fmt.Sprintf("%s", out)
	//fmt.Println(output)
	if err != nil {
		panic(output)
	}

}
