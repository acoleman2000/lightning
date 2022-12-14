// Copyright (C) The Lightning Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package lightning

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kshedden/statmodel/glm"
	"github.com/kshedden/statmodel/statmodel"
	"gopkg.in/check.v1"
)

type glmSuite struct{}

var _ = check.Suite(&glmSuite{})

func (s *glmSuite) TestFit(c *check.C) {
	data := [][]statmodel.Dtype{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 0, 0, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 0, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 0, 0, 0, 1, 0, 1, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 0, 1, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 1},
		{17.99, 20.57, 19.69, 11.42, 20.29, 12.45, 18.25, 13.71, 13.0, 12.46, 16.02, 15.78, 19.17, 15.85, 13.73, 14.54, 14.68, 16.13, 19.81, 13.54, 13.08, 9.504, 15.34, 21.16, 16.65, 17.14, 14.58, 18.61, 15.3, 17.57, 18.63, 11.84, 17.02, 19.27, 16.13, 16.74, 14.25, 13.03, 14.99, 13.48, 13.44, 10.95, 19.07, 13.28, 13.17, 18.65, 8.196, 13.17, 12.05, 13.49, 11.76, 13.64, 11.94, 18.22, 15.1, 11.52, 19.21, 14.71, 13.05, 8.618, 10.17, 8.598, 14.25, 9.173, 12.68, 14.78, 9.465, 11.31, 9.029, 12.78, 18.94, 8.888, 17.2, 13.8, 12.31, 16.07, 13.53, 18.05, 20.18, 12.86, 11.45, 13.34, 25.22, 19.1, 12.0, 18.46, 14.48, 19.02, 12.36, 14.64, 14.62, 15.37, 13.27, 13.45, 15.06, 20.26, 12.18, 9.787, 11.6, 14.42, 13.61, 6.981, 12.18, 9.876, 10.49, 13.11, 11.64, 12.36, 22.27, 11.34, 9.777, 12.63, 14.26, 10.51, 8.726, 11.93, 8.95, 14.87, 15.78, 17.95, 11.41, 18.66, 24.25, 14.5, 13.37, 13.85, 13.61, 19.0, 15.1, 19.79, 12.19, 15.46, 16.16, 15.71, 18.45, 12.77, 11.71, 11.43, 14.95, 11.28, 9.738, 16.11, 11.43, 12.9, 10.75, 11.9, 11.8, 14.95, 14.44, 13.74, 13.0, 8.219, 9.731, 11.15, 13.15, 12.25, 17.68, 16.84, 12.06, 10.9, 11.75, 19.19, 19.59, 12.34, 23.27, 14.97, 10.8, 16.78, 17.47, 14.97, 12.32, 13.43, 15.46, 11.08, 10.66, 8.671, 9.904, 16.46, 13.01, 12.81, 27.22, 21.09, 15.7, 11.41, 15.28, 10.08, 18.31, 11.71, 11.81, 12.3, 14.22, 12.77, 9.72, 12.34, 14.86, 12.91, 13.77, 18.08, 19.18, 14.45, 12.23, 17.54, 23.29, 13.81, 12.47, 15.12, 9.876, 17.01, 13.11, 15.27, 20.58, 11.84, 28.11, 17.42, 14.19, 13.86, 11.89, 10.2, 19.8, 19.53, 13.65, 13.56, 10.18, 15.75, 13.27, 14.34, 10.44, 15.0, 12.62, 12.83, 17.05, 11.32, 11.22, 20.51, 9.567, 14.03, 23.21, 20.48, 14.22, 17.46, 13.64, 12.42, 11.3, 13.75, 19.4, 10.48, 13.2, 12.89, 10.65, 11.52, 20.94, 11.5, 19.73, 17.3, 19.45, 13.96, 19.55, 15.32, 15.66, 15.53, 20.31, 17.35, 17.29, 15.61, 17.19, 20.73, 10.6, 13.59, 12.87, 10.71, 14.29, 11.29, 21.75, 9.742, 17.93, 11.89, 11.33, 18.81, 13.59, 13.85, 19.16, 11.74, 19.4, 16.24, 12.89, 12.58, 11.94, 12.89, 11.26, 11.37, 14.41, 14.96, 12.95, 11.85, 12.72, 13.77, 10.91, 11.76, 14.26, 10.51, 19.53, 12.46, 20.09, 10.49, 11.46, 11.6, 13.2, 9.0, 13.5, 13.05, 11.7, 14.61, 12.76, 11.54, 8.597, 12.49, 12.18, 18.22, 9.042, 12.43, 10.25, 20.16, 12.86, 20.34, 12.2, 12.67, 14.11, 12.03, 16.27, 16.26, 16.03, 12.98, 11.22, 11.25, 12.3, 17.06, 12.99, 18.77, 10.05, 23.51, 14.42, 9.606, 11.06, 19.68, 11.71, 10.26, 12.06, 14.76, 11.47, 11.95, 11.66, 15.75, 25.73, 15.08, 11.14, 12.56, 13.05, 13.87, 8.878, 9.436, 12.54, 13.3, 12.76, 16.5, 13.4, 20.44, 20.2, 12.21, 21.71, 22.01, 16.35, 15.19, 21.37, 20.64, 13.69, 16.17, 10.57, 13.46, 13.66, 11.08, 11.27, 11.04, 12.05, 12.39, 13.28, 14.6, 12.21, 13.88, 11.27, 19.55, 10.26, 8.734, 15.49, 21.61, 12.1, 14.06, 13.51, 12.8, 11.06, 11.8, 17.91, 11.93, 12.96, 12.94, 12.34, 10.94, 16.14, 12.85, 17.99, 12.27, 11.36, 11.04, 9.397, 14.99, 15.13, 11.89, 9.405, 15.5, 12.7, 11.16, 11.57, 14.69, 11.61, 13.66, 9.742, 10.03, 10.48, 10.8, 11.13, 12.72, 14.9, 12.4, 20.18, 18.82, 14.86, 13.98, 12.87, 14.04, 13.85, 14.02, 10.97, 17.27, 13.78, 10.57, 18.03, 11.99, 17.75, 14.8, 14.53, 21.1, 11.87, 19.59, 12.0, 14.53, 12.62, 13.38, 11.63, 13.21, 13.0, 9.755, 17.08, 27.42, 14.4, 11.6, 13.17, 13.24, 13.14, 9.668, 17.6, 11.62, 9.667, 12.04, 14.92, 12.27, 10.88, 12.83, 14.2, 13.9, 11.49, 16.25, 12.16, 13.9, 13.47, 13.7, 15.73, 12.45, 14.64, 19.44, 11.68, 16.69, 12.25, 17.85, 18.01, 12.46, 13.16, 14.87, 12.65, 12.47, 18.49, 20.59, 15.04, 13.82, 12.54, 23.09, 9.268, 9.676, 12.22, 11.06, 16.3, 15.46, 11.74, 14.81, 13.4, 14.58, 15.05, 11.34, 18.31, 19.89, 12.88, 12.75, 9.295, 24.63, 11.26, 13.71, 9.847, 8.571, 13.46, 12.34, 13.94, 12.07, 11.75, 11.67, 13.68, 20.47, 10.96, 20.55, 14.27, 11.69, 7.729, 7.691, 11.54, 14.47, 14.74, 13.21, 13.87, 13.62, 10.32, 10.26, 9.683, 10.82, 10.86, 11.13, 12.77, 9.333, 12.88, 10.29, 10.16, 9.423, 14.59, 11.51, 14.05, 11.2, 15.22, 20.92, 21.56, 20.13, 16.6, 20.6, 7.76},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}
	dataset := statmodel.NewDataset(data, []string{"y", "x1", "const"})
	model, err := glm.NewGLM(dataset, "y", []string{"const", "x1"}, glmConfig)
	c.Assert(err, check.IsNil)
	result := model.Fit()
	c.Logf("%s", result.Summary())
	c.Logf("VCov\t%v", result.VCov())
	c.Logf("Params\t%v", result.Params())
	c.Logf("StdErr\t%v", result.StdErr())
	c.Logf("ZScores\t%v", result.ZScores())
	c.Logf("LogLike\t%v", result.LogLike())
	expect := -165.00542199378245 // from python
	c.Check(math.Abs(result.LogLike()-expect) < 0.00000000001, check.Equals, true)
}

var pyImports = `
import scipy
import statsmodels.formula.api as smf
import statsmodels.api as sm
import numpy as np
import pandas as pd
`

func checkVirtualenv(c *check.C) {
	cmd := exec.Command("python3", "-")
	cmd.Stdin = strings.NewReader(pyImports)
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.Logf("%s", out)
		c.Skip("test requires python virtualenv with libraries installed")
	}
}

func (s *glmSuite) TestFitDivergeFromPython(c *check.C) {
	checkVirtualenv(c)
	c.Skip("slow test")
	data0 := [][]statmodel.Dtype{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 2, 1, 4, 3, 6, 5, 7, 8, 9},
	}
	data1 := [][]statmodel.Dtype{
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 0, 0, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 0, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 0, 0, 0, 1, 0, 1, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 0, 1, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 0, 1, 0, 1, 0, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 1},
		{17.99, 20.57, 19.69, 11.42, 20.29, 12.45, 18.25, 13.71, 13.0, 12.46, 16.02, 15.78, 19.17, 15.85, 13.73, 14.54, 14.68, 16.13, 19.81, 13.54, 13.08, 9.504, 15.34, 21.16, 16.65, 17.14, 14.58, 18.61, 15.3, 17.57, 18.63, 11.84, 17.02, 19.27, 16.13, 16.74, 14.25, 13.03, 14.99, 13.48, 13.44, 10.95, 19.07, 13.28, 13.17, 18.65, 8.196, 13.17, 12.05, 13.49, 11.76, 13.64, 11.94, 18.22, 15.1, 11.52, 19.21, 14.71, 13.05, 8.618, 10.17, 8.598, 14.25, 9.173, 12.68, 14.78, 9.465, 11.31, 9.029, 12.78, 18.94, 8.888, 17.2, 13.8, 12.31, 16.07, 13.53, 18.05, 20.18, 12.86, 11.45, 13.34, 25.22, 19.1, 12.0, 18.46, 14.48, 19.02, 12.36, 14.64, 14.62, 15.37, 13.27, 13.45, 15.06, 20.26, 12.18, 9.787, 11.6, 14.42, 13.61, 6.981, 12.18, 9.876, 10.49, 13.11, 11.64, 12.36, 22.27, 11.34, 9.777, 12.63, 14.26, 10.51, 8.726, 11.93, 8.95, 14.87, 15.78, 17.95, 11.41, 18.66, 24.25, 14.5, 13.37, 13.85, 13.61, 19.0, 15.1, 19.79, 12.19, 15.46, 16.16, 15.71, 18.45, 12.77, 11.71, 11.43, 14.95, 11.28, 9.738, 16.11, 11.43, 12.9, 10.75, 11.9, 11.8, 14.95, 14.44, 13.74, 13.0, 8.219, 9.731, 11.15, 13.15, 12.25, 17.68, 16.84, 12.06, 10.9, 11.75, 19.19, 19.59, 12.34, 23.27, 14.97, 10.8, 16.78, 17.47, 14.97, 12.32, 13.43, 15.46, 11.08, 10.66, 8.671, 9.904, 16.46, 13.01, 12.81, 27.22, 21.09, 15.7, 11.41, 15.28, 10.08, 18.31, 11.71, 11.81, 12.3, 14.22, 12.77, 9.72, 12.34, 14.86, 12.91, 13.77, 18.08, 19.18, 14.45, 12.23, 17.54, 23.29, 13.81, 12.47, 15.12, 9.876, 17.01, 13.11, 15.27, 20.58, 11.84, 28.11, 17.42, 14.19, 13.86, 11.89, 10.2, 19.8, 19.53, 13.65, 13.56, 10.18, 15.75, 13.27, 14.34, 10.44, 15.0, 12.62, 12.83, 17.05, 11.32, 11.22, 20.51, 9.567, 14.03, 23.21, 20.48, 14.22, 17.46, 13.64, 12.42, 11.3, 13.75, 19.4, 10.48, 13.2, 12.89, 10.65, 11.52, 20.94, 11.5, 19.73, 17.3, 19.45, 13.96, 19.55, 15.32, 15.66, 15.53, 20.31, 17.35, 17.29, 15.61, 17.19, 20.73, 10.6, 13.59, 12.87, 10.71, 14.29, 11.29, 21.75, 9.742, 17.93, 11.89, 11.33, 18.81, 13.59, 13.85, 19.16, 11.74, 19.4, 16.24, 12.89, 12.58, 11.94, 12.89, 11.26, 11.37, 14.41, 14.96, 12.95, 11.85, 12.72, 13.77, 10.91, 11.76, 14.26, 10.51, 19.53, 12.46, 20.09, 10.49, 11.46, 11.6, 13.2, 9.0, 13.5, 13.05, 11.7, 14.61, 12.76, 11.54, 8.597, 12.49, 12.18, 18.22, 9.042, 12.43, 10.25, 20.16, 12.86, 20.34, 12.2, 12.67, 14.11, 12.03, 16.27, 16.26, 16.03, 12.98, 11.22, 11.25, 12.3, 17.06, 12.99, 18.77, 10.05, 23.51, 14.42, 9.606, 11.06, 19.68, 11.71, 10.26, 12.06, 14.76, 11.47, 11.95, 11.66, 15.75, 25.73, 15.08, 11.14, 12.56, 13.05, 13.87, 8.878, 9.436, 12.54, 13.3, 12.76, 16.5, 13.4, 20.44, 20.2, 12.21, 21.71, 22.01, 16.35, 15.19, 21.37, 20.64, 13.69, 16.17, 10.57, 13.46, 13.66, 11.08, 11.27, 11.04, 12.05, 12.39, 13.28, 14.6, 12.21, 13.88, 11.27, 19.55, 10.26, 8.734, 15.49, 21.61, 12.1, 14.06, 13.51, 12.8, 11.06, 11.8, 17.91, 11.93, 12.96, 12.94, 12.34, 10.94, 16.14, 12.85, 17.99, 12.27, 11.36, 11.04, 9.397, 14.99, 15.13, 11.89, 9.405, 15.5, 12.7, 11.16, 11.57, 14.69, 11.61, 13.66, 9.742, 10.03, 10.48, 10.8, 11.13, 12.72, 14.9, 12.4, 20.18, 18.82, 14.86, 13.98, 12.87, 14.04, 13.85, 14.02, 10.97, 17.27, 13.78, 10.57, 18.03, 11.99, 17.75, 14.8, 14.53, 21.1, 11.87, 19.59, 12.0, 14.53, 12.62, 13.38, 11.63, 13.21, 13.0, 9.755, 17.08, 27.42, 14.4, 11.6, 13.17, 13.24, 13.14, 9.668, 17.6, 11.62, 9.667, 12.04, 14.92, 12.27, 10.88, 12.83, 14.2, 13.9, 11.49, 16.25, 12.16, 13.9, 13.47, 13.7, 15.73, 12.45, 14.64, 19.44, 11.68, 16.69, 12.25, 17.85, 18.01, 12.46, 13.16, 14.87, 12.65, 12.47, 18.49, 20.59, 15.04, 13.82, 12.54, 23.09, 9.268, 9.676, 12.22, 11.06, 16.3, 15.46, 11.74, 14.81, 13.4, 14.58, 15.05, 11.34, 18.31, 19.89, 12.88, 12.75, 9.295, 24.63, 11.26, 13.71, 9.847, 8.571, 13.46, 12.34, 13.94, 12.07, 11.75, 11.67, 13.68, 20.47, 10.96, 20.55, 14.27, 11.69, 7.729, 7.691, 11.54, 14.47, 14.74, 13.21, 13.87, 13.62, 10.32, 10.26, 9.683, 10.82, 10.86, 11.13, 12.77, 9.333, 12.88, 10.29, 10.16, 9.423, 14.59, 11.51, 14.05, 11.2, 15.22, 20.92, 21.56, 20.13, 16.6, 20.6, 7.76},
	}
	for i := 0; i <= len(data1[0]); i++ {
		c.Logf("================== %d", i)
		data := [][]statmodel.Dtype{
			append([]statmodel.Dtype(nil), data0[0]...),
			append([]statmodel.Dtype(nil), data0[1]...),
		}
		for j := 0; j < i; j++ {
			if len(data[0]) <= j {
				data[0] = append(data[0], data1[0][j])
				data[1] = append(data[1], data1[1][j])
			} else {
				data[0][j] = data1[0][j]
				data[1][j] = data1[1][j]
			}
		}
		constants := make([]statmodel.Dtype, len(data[0]))
		for i := range constants {
			constants[i] = 1
		}
		data = append(data, constants)
		c.Logf("%v", data)

		dataset := statmodel.NewDataset(data, []string{"y", "x1", "C"})
		model, err := glm.NewGLM(dataset, "y", []string{"x1", "C"}, glmConfig)
		c.Assert(err, check.IsNil)
		result := model.Fit()
		c.Logf("%s", result.Summary())
		c.Logf("%v", result.LogLike())

		pydata := "["
		for row, values := range data[:2] {
			if row > 0 {
				pydata += ","
			}
			pydata += "\n    ["
			for col, v := range values {
				if col > 0 {
					pydata += ", "
				}
				pydata += fmt.Sprintf("%v", v)
			}
			pydata += "]"
		}
		pydata += "]"
		py := pyImports + `
data = np.array(` + pydata + `)
df = pd.DataFrame(data.T, columns=['y','x1'])
fit = smf.glm('y ~ x1', family=sm.families.Binomial(), data=df).fit()
print(fit.summary())
print(fit.llf)
`
		cmd := exec.Command("python3", "-")
		cmd.Stdin = strings.NewReader(py)
		out, err := cmd.CombinedOutput()
		c.Logf("%s", out)
		c.Assert(err, check.IsNil)
		outlines := bytes.Split(out, []byte{'\n'})
		llf, err := strconv.ParseFloat(string(outlines[len(outlines)-2]), 64)
		c.Assert(err, check.IsNil)
		c.Assert(math.Abs(result.LogLike()-llf) < 0.000000000001, check.Equals, true)
	}
}

func (s *glmSuite) TestPvalueRealDataVsPython(c *check.C) {
	checkVirtualenv(c)
	samples, err := loadSampleInfo("glm_test_samples.csv")
	if err != nil {
		c.Skip("test requires glm_test_samples.csv (not included)")
	}
	c.Logf("Nsamples = %d", len(samples))
	nPCA := 5
	// data series: y, rand, pca1, ..., pcaN
	data := [][]statmodel.Dtype{nil, nil}
	for i := 0; i < nPCA; i++ {
		data = append(data, []statmodel.Dtype(nil))
	}
	onehot := []bool{}
	for _, si := range samples {
		if !si.isTraining {
			continue
		}
		if si.isCase {
			data[0] = append(data[0], 1)
		} else {
			data[0] = append(data[0], 0)
		}
		r := rand.Int()&1 == 1
		if rand.Int()&0x1f == 0 {
			// 1/32 samples have onehot==outcome, the rest
			// are random
			r = si.isCase
		}
		onehot = append(onehot, r)
		if r {
			data[1] = append(data[1], 1)
		} else {
			data[1] = append(data[1], 0)
		}
		for i := 0; i < nPCA; i++ {
			data[i+2] = append(data[i+2], si.pcaComponents[i])
		}
	}

	pGo := glmPvalueFunc(samples, nPCA)(onehot)
	c.Logf("pGo = %g", pGo)

	var pydata bytes.Buffer
	pydata.WriteString("[")
	for row, values := range data {
		if row > 0 {
			pydata.WriteString(",")
		}
		pydata.WriteString("\n    [")
		for col, v := range values {
			if col > 0 {
				pydata.WriteString(", ")
			}
			fmt.Fprintf(&pydata, "%v", v)
		}
		pydata.WriteString("]")
	}
	pydata.WriteString("]")
	py := pyImports + `
data = np.array(` + pydata.String() + `)
columns = ['y','onehot']
formula = ''
for i in range(` + fmt.Sprintf("%d", nPCA) + `):
    columns.append('x'+str(i+1))
    if len(formula) > 0:
        formula += ' + '
    formula += 'x'+str(i+1)
df = pd.DataFrame(data.T, columns=columns)

mod1 = smf.glm('y ~ '+formula, family=sm.families.Binomial(), data=df).fit()
# print(mod1.summary())
print('mod1.llf = ', mod1.llf)

mod2 = smf.glm('y ~ onehot + '+formula, family=sm.families.Binomial(), data=df).fit()
# print(mod2.summary())
print('mod2.llf = ', mod2.llf)

df = 1
p = 1 - scipy.stats.chi2.cdf(-2 * (mod1.llf - mod2.llf), df)
print(p)
`
	c.Logf("python...")
	cmd := exec.Command("python3", "-")
	cmd.Stdin = strings.NewReader(py)
	out, err := cmd.CombinedOutput()
	c.Logf("%s", out)
	c.Assert(err, check.IsNil)
	outlines := bytes.Split(out, []byte{'\n'})
	pPy, err := strconv.ParseFloat(string(outlines[len(outlines)-2]), 64)
	c.Assert(err, check.IsNil)
	c.Logf("pPy = %g", pPy)
	c.Assert(math.Abs(pGo-pPy) < 0.000001, check.Equals, true)
}

func (s *glmSuite) TestPvalue(c *check.C) {
	// csv: casecontrol,onehot,pca1,pca2,...
	csv2test := func(csv string) (samples []sampleInfo, onehot []bool, npca int) {
		for _, line := range strings.Split(csv, "\n") {
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			fields := strings.Split(line, ",")
			var pca []float64
			for _, s := range fields[2:] {
				f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
				c.Assert(err, check.IsNil)
				pca = append(pca, f)
			}
			isCase := strings.TrimSpace(fields[0]) == "1"
			samples = append(samples, sampleInfo{
				isCase:        isCase,
				isControl:     !isCase,
				isTraining:    true,
				pcaComponents: pca,
			})
			onehot = append(onehot, strings.TrimSpace(fields[1]) == "1")
			if rand.Int()%5 == 0 {
				samples = append(samples, sampleInfo{
					isCase:        rand.Int()%2 == 0,
					isValidation:  true,
					pcaComponents: pca,
				})
			}
		}
		npca = len(samples[0].pcaComponents)
		return
	}

	samples, onehot, npca := csv2test(`
# case=1, onehot=1, pca1, pca2, pca3
0, 0, 1, 1.21, 2.37
0, 0, 2, 1.22, 2.38
0, 0, 3, 1.23, 2.39
0, 0, 1, 1.24, 2.33
0, 0, 2, 1.25, 2.34
1, 1, 3, 1.26, 2.35
1, 1, 1, 1.23, 2.36
1, 1, 2, 1.22, 2.32
1, 1, 3, 1.21, 2.31
`)
	c.Check(glmPvalueFunc(samples, npca)(onehot), check.Equals, 0.002789665435066107)
}

var benchSamples, benchOnehot = func() ([]sampleInfo, []bool) {
	pcaComponents := 10
	samples := []sampleInfo{}
	onehot := []bool{}
	r := make([]float64, pcaComponents)
	for j := 0; j < 10000; j++ {
		for i := 0; i < len(r); i++ {
			r[i] = rand.Float64()
		}
		samples = append(samples, sampleInfo{
			id:            fmt.Sprintf("sample%d", j),
			isCase:        j%2 == 0 && j > 200,
			isControl:     j%2 == 1 || j <= 200,
			isTraining:    true,
			pcaComponents: append([]float64(nil), r...),
		})
		onehot = append(onehot, j%2 == 0)
	}
	return samples, onehot
}()

func (s *glmSuite) BenchmarkPvalue(c *check.C) {
	for i := 0; i < c.N; i++ {
		p := glmPvalueFunc(benchSamples, len(benchSamples[0].pcaComponents))(benchOnehot)
		c.Check(p, check.Equals, 0.0)
	}
}
