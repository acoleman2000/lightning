# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

cwlVersion: v1.2
class: ExpressionTool
label: Create list of VCFs and sample names
hints:
  LoadListingRequirement:
    loadListing: shallow_listing
inputs:
  vcfs:
    type: File[]
    label: Input VCFs
outputs:
  sampleids:
    type: string[]
    label: Sample names of VCFs
requirements:
  InlineJavascriptRequirement: {}
expression: |
  ${
    var samples = [];
    for (var i = 0; i < inputs.vcfs.length; i++) {
      var file = inputs.vcfs[i];
      if (file.nameext == ".vcf") {
        var sample = file.basename.split(".").slice(0, -1).join(".");
        samples.push(sample);
      }
      if (file.nameext == ".gz") {
        var sample = file.basename.split(".").slice(0, -2).join(".");
        samples.push(sample);
      }
    }
    return {"sampleids": samples};
  }