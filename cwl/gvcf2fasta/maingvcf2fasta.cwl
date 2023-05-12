# Copyright (C) The Lightning Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

$namespaces:
  arv: "http://arvados.org/cwl#"
cwlVersion: v1.1
class: Workflow
label: Scatter to Convert various gVCF to FASTA
requirements:
  SubworkflowFeatureRequirement: {}
  ScatterFeatureRequirement: {}
hints:
  DockerRequirement:
    dockerPull: vcfutil
  arv:IntermediateOutput:
    outputTTL: 604800
  
inputs:
  vcfsinput:
    - type: record
      vcfsdir:
        type: Directory
        label: Input directory of VCFs
    - type: record
      vcfs:
        type: File[]
        label: Input VCFs in array of files 
    - type: record
      vcftars:
        type: File[]
        label: Input VCF tars
  genomebed:
    type: File
    label: Whole genome BED
  ref:
    type: File
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
    when: $(inputs.sampleid && inputs.vcfsinput.vcf)
    scatter: [sampleid, vcf]
    scatterMethod: dotproduct
    in:
      sampleid: sampleids
      vcf: vcfsinput
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]

  gvcf2fasta_splitvcf-imputation-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcf-imputation-wf.cwl
    when: $(inputs.sampleids && inputs.splitvcfdirs && inputs.chrs && inputs.refsdir && inputs.mapsdir && inputs.panelcallbed && inputs.panelnocallbed)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: sampleids
      splitvcfdir: vcfsinput
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
    when: $(inputs.sampleid && inputs.splitvcfdir)
    scatter: [sampleid, splitvcfdir]
    scatterMethod: dotproduct
    in:
      sampleid: sampleids
      splitvcfdir: vcfsinput
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]
  gvcf2fasta_splitvcftar-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta_splitvcftar-wf.cwl
    when: $(inputs.sampleids && inputs.vcftars)
    scatter: [sampleid, vcftar]
    scatterMethod: dotproduct
    in:
      sampleid: sampleids
      vcftar: vcfsinput
      gqcutoff: gqcutoff
      genomebed: genomebed
      ref: ref
    out: [fas]
  getfiles:
    run: subworkflows/scatter/helpers/getfiles.cwl
    when: $(inputs.vcfsinput && inputs.sampleid === null && inputs.sampleids === null)
    in:
      dir: vcfsinput
    out: [vcfs, samples]
  gvcf2fasta-wf:
    run: subworkflows/scatter/gvcf2fasta/gvcf2fasta-wf.cwl
    scatter: [sampleid, vcf]
    when: $(getfiles.vcfs and getfiles.samples)
    scatterMethod: dotproduct
    in:
      sampleid: getfiles/samples
      vcf: getfiles/vcfs
      genomebed: genomebed
      ref: ref
    out: [fas]

