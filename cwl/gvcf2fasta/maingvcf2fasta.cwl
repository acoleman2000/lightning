# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

$namespaces:
  arv: "http://arvados.org/cwl#"
cwlVersion: v1.2
class: Workflow
label: Scatter to Convert various gVCF to FASTA
requirements:
  SubworkflowFeatureRequirement: {}
  ScatterFeatureRequirement: {}
  MultipleInputFeatureRequirement: {}
  InlineJavascriptRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil
  arv:IntermediateOutput:
    outputTTL: 604800
  
inputs:
  splitvcfdirs:
    type: Directory[]?
    label: Input directory of split gVCFs
  vcfsdir:
    type: Directory?
    label: Input directory of VCFs
  vcfs:
    type: File[]?
    label: Input VCFs in array of files 
  vcftars:
    type: File[]?
    label: Input VCF tars
  genomebed:
    type: File?
    label: Whole genome BED
  ref:
    type: File?
    label: Reference FASTA
  gqcutoff:
    type: int?
    label: GQ (Genotype Quality) cutoff for filtering
  sampleids:
    type: string[]?
    label: Sample IDs
  chrs: string[]?
  refsdir: Directory?
  mapsdir: Directory?
  panelnocallbed: File?
  panelcallbed: File?


outputs:
  fas:
    type:
      type: array
      items:
        type: array
        items: File
    label: Output pairs of FASTAs
    outputSource: 
      - gvcf2fasta_nonrefvcf-wf/fas
      - gvcf2fasta_splitvcf-imputation-wf/fas
      - gvcf2fasta_splitvcf-wf/fas
      - gvcf2fasta_splitvcftar-wf/fas
      - gvcf2fasta-wf/fas
    pickValue: first_non_null

steps: 
  gvcf2fasta_nonrefvcf-wf:
    run:  subworkflows/scatter/gvcf2fasta/gvcf2fasta_nonrefvcf-wf.cwl
    when: $(inputs.sampleid && inputs.vcf)
    scatter: [sampleid, vcf]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: sampleids
        default: []
      vcf: 
        source: vcfs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]

  gvcf2fasta_splitvcf-imputation-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcf-imputation-wf.cwl
    when: $(inputs.sampleids !== null  && inputs.splitvcfdirs  !== null && inputs.chrs !== null && inputs.refsdir !== null && inputs.mapsdir !== null && inputs.panelcallbed  !== null && inputs.panelnocallbed !== null)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: sampleids
        default: []
      splitvcfdir: 
        source: splitvcfdirs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
      chrs: chrs
      refsdir: refsdir
      mapsdir: mapsdir
      panelnocallbed: panelnocallbed
      panelcallbed: panelcallbed
    out: [fas]

  gvcf2fasta_splitvcf-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcf-wf.cwl
    when: $(inputs.sampleid !== null && inputs.splitvcfdir !== null && inputs.chrs == null)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: sampleids
        default: []
      splitvcfdir: 
        source: splitvcfdirs
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]
  gvcf2fasta_splitvcftar-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcftar-wf.cwl
    when: $(inputs.sampleids !== null  && inputs.vcftars !== null )
    scatter: [sampleid, vcftar]
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: sampleids
        default: []
      vcftar: 
        source: vcftars
        default: []
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]
  getfiles:
    run: subworkflows/scatter/helpers/getfiles.cwl
    when: $(inputs.vcfsdir !== null) #  && inputs.sampleid === null && inputs.sampleids === null
    in:
      dir: vcfsdir
    out: [vcfs, samples]
  gvcf2fasta-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta-wf.cwl
    scatter: [sampleid, vcf]
    when: $(inputs.vcfsdir !== null)
    scatterMethod: dotproduct
    in:
      sampleid: 
        source: getfiles/samples
        default: []
      vcf: 
        source: getfiles/vcfs
        default: []
      genomebed: genomebed
      ref: ref
      gqcutoff: gqcutoff
    out: [fas]

