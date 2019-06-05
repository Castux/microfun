using System;
using System.Collections.Generic;

class Program
{
    static int Main(string[] args)
    {
        // Get arguments from command line

        if (args.Length < 1)
        {
            Console.WriteLine("Invalid arguments");
            PrintUsage();
            return 1;
        }

        // Lex files

        var tokens = new List<Token>();
        var success = true;

        foreach(var path in args)
        {
            success = LexFile(path, tokens) && success;
        }

        if (!success)
        {
            Console.WriteLine("Failure");
            return 1;
        }

        // Parse stream of tokens



        Console.WriteLine("Success");
        return 0;
    }

    private static bool LexFile(string path, List<Token> tokens)
    {
        // Load file

        var file = new SourceFile(path);
        var success = file.Load();

        if (!success)
        {
            Console.WriteLine("Could not load file " + path);
            return false;
        }

        // Lex file

        var lexer = new Lexer(file);
        success = lexer.Lex(tokens);

        if (!success)
        {
            lexer.Report.Print();
        }

        return success;
    }

    static void PrintUsage()
    {
        Console.WriteLine("usage: microfun source1.mf source2.mf ...");
    }
}